package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/shohag/piperelay/internal/api"
	"github.com/shohag/piperelay/internal/config"
	"github.com/shohag/piperelay/internal/delivery"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

var version = "0.1.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "piperelay",
		Short: "PipeRelay — Self-hosted webhook delivery system",
	}

	var configPath string
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file")

	rootCmd.AddCommand(serveCmd(&configPath))
	rootCmd.AddCommand(migrateCmd(&configPath))
	rootCmd.AddCommand(appCmd(&configPath))
	rootCmd.AddCommand(statsCmd(&configPath))
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the PipeRelay server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log := setupLogger(cfg.Logging)

			store, err := setupStorage(cfg.Storage, log)
			if err != nil {
				return fmt.Errorf("failed to setup storage: %w", err)
			}
			defer store.Close()

			if err := store.Migrate(context.Background()); err != nil {
				return fmt.Errorf("failed to run migrations: %w", err)
			}
			log.Info().Msg("database migrations completed")

			pool := delivery.NewPool(cfg.Delivery, store, log)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			pool.Start(ctx)

			server := api.NewServer(cfg.Server, store, log)
			go func() {
				if err := server.Start(); err != nil && err != http.ErrServerClosed {
					log.Fatal().Err(err).Msg("server error")
				}
			}()

			log.Info().
				Str("version", version).
				Int("port", cfg.Server.Port).
				Int("workers", cfg.Delivery.Workers).
				Str("storage", cfg.Storage.Driver).
				Msg("PipeRelay is running")

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			log.Info().Msg("shutting down...")

			if err := server.Shutdown(10 * time.Second); err != nil {
				log.Error().Err(err).Msg("server shutdown error")
			}

			pool.Stop()

			log.Info().Msg("PipeRelay stopped")
			return nil
		},
	}
}

func migrateCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log := setupLogger(cfg.Logging)

			store, err := setupStorage(cfg.Storage, log)
			if err != nil {
				return fmt.Errorf("failed to setup storage: %w", err)
			}
			defer store.Close()

			if err := store.Migrate(context.Background()); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			log.Info().Msg("migrations completed successfully")
			return nil
		},
	}
}

func appCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage applications",
	}

	// app create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new application",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			store, cleanup, err := storeFromConfig(*configPath)
			if err != nil {
				return err
			}
			defer cleanup()

			now := time.Now().UTC()
			app := &models.Application{
				ID:        models.NewID("app"),
				Name:      name,
				APIKey:    models.NewAPIKey(),
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := store.CreateApplication(context.Background(), app); err != nil {
				return fmt.Errorf("failed to create application: %w", err)
			}

			out, _ := json.MarshalIndent(app, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
	createCmd.Flags().String("name", "", "application name")

	// app list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := storeFromConfig(*configPath)
			if err != nil {
				return err
			}
			defer cleanup()

			apps, err := store.ListApplications(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list applications: %w", err)
			}

			if len(apps) == 0 {
				fmt.Println("No applications found.")
				return nil
			}

			for _, app := range apps {
				fmt.Printf("  %s  %s  (created %s)\n", app.ID, app.Name, app.CreatedAt.Format(time.RFC3339))
			}
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd)
	return cmd
}

func statsCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show delivery stats for an application",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: piperelay stats <app_id>")
			}

			store, cleanup, err := storeFromConfig(*configPath)
			if err != nil {
				return err
			}
			defer cleanup()

			stats, err := store.GetStats(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get stats: %w", err)
			}

			out, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("PipeRelay v%s\n", version)
		},
	}
}

func setupLogger(cfg config.LoggingConfig) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.Format == "console" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
			With().Timestamp().Logger()
	}
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func setupStorage(cfg config.StorageConfig, log zerolog.Logger) (storage.Storage, error) {
	switch cfg.Driver {
	case "sqlite":
		log.Info().Str("path", cfg.SQLite.Path).Msg("using SQLite storage")
		return storage.NewSQLite(cfg.SQLite.Path)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.Driver)
	}
}

func storeFromConfig(configPath string) (storage.Storage, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	log := setupLogger(cfg.Logging)
	store, err := setupStorage(cfg.Storage, log)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup storage: %w", err)
	}

	if err := store.Migrate(context.Background()); err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, func() { store.Close() }, nil
}
