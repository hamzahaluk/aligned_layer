package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yetanotherco/aligned_layer/aggregator/pkg"
	"github.com/yetanotherco/aligned_layer/core/config"
)

var (
	Version   string // Version is the version of the binary.
	GitCommit string
	GitDate   string
)

func main() {
	app := createCLIApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}

// createCLIApp initializes the CLI application.
func createCLIApp() *cli.App {
	return &cli.App{
		Name:        "aligned-layer-aggregator",
		Usage:       "Service that aggregates signed responses from operator nodes.",
		Description: "This service listens for tasks and aggregates operator node responses.",
		Version:     fmt.Sprintf("%s-%s-%s", Version, GitCommit, GitDate),
		Flags:       []cli.Flag{config.ConfigFileFlag},
		Action:      aggregatorMain,
	}
}

// aggregatorMain is the main action for the CLI application.
func aggregatorMain(ctx *cli.Context) error {
	configFilePath := ctx.String(config.ConfigFileFlag.Name)
	aggregatorConfig := config.NewAggregatorConfig(configFilePath)

	aggregator, err := initializeAggregator(aggregatorConfig)
	if err != nil {
		return err
	}

	startBackgroundProcesses(aggregator)

	return aggregator.Start(context.Background())
}

// initializeAggregator creates a new Aggregator instance.
func initializeAggregator(cfg *config.AggregatorConfig) (*pkg.Aggregator, error) {
	aggregator, err := pkg.NewAggregator(*cfg)
	if err != nil {
		cfg.BaseConfig.Logger.Error("Failed to create aggregator", "err", err)
		return nil, fmt.Errorf("failed to create aggregator: %w", err)
	}
	return aggregator, nil
}

// startBackgroundProcesses handles background tasks for the aggregator.
func startBackgroundProcesses(aggregator *pkg.Aggregator) {
	go startGarbageCollector(aggregator)
	go subscribeToTasks(aggregator)
}

// startGarbageCollector periodically clears tasks.
func startGarbageCollector(aggregator *pkg.Aggregator) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Garbage collector panicked, restarting:", r)
			go startGarbageCollector(aggregator)
		}
	}()
	for {
		log.Println("Running garbage collector...")
		aggregator.ClearTasksFromMaps()
	}
}

// subscribeToTasks listens for new tasks from the service manager.
func subscribeToTasks(aggregator *pkg.Aggregator) {
	if err := aggregator.SubscribeToNewTasks(); err != nil {
		aggregator.Config.BaseConfig.Logger.Fatal("Failed to subscribe to new tasks", "err", err)
	}
}
