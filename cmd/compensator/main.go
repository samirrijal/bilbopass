package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/samirrijal/bilbopass/internal/core/usecases"
	"github.com/samirrijal/bilbopass/internal/pkg/config"
	"github.com/samirrijal/bilbopass/internal/workflows"
)

func main() {
	cfg, err := config.Load("bilbopass-compensator")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	_ = cfg // used for DB connections in activities

	// Connect to Temporal
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalf("temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, "compensation-queue", worker.Options{})

	// Register workflow & activities
	w.RegisterWorkflow(workflows.CompensationWorkflow)
	w.RegisterActivity(&workflows.CompensationActivities{
		// In production, inject real service implementations here.
		CompensationService: &usecases.CompensationService{},
	})

	log.Println("compensator worker started")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("worker: %v", err)
	}
}
