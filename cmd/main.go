package main

import (
	"log"
	"os"
	"time"

	"github.com/sysdiglabs/ekscloudwatch"
)

func main() {
	log.Printf("Cloudwatch EKS log started")
	clusterNameOverride := os.Getenv("CLUSTER_NAME")
	endpoint := os.Getenv("ENDPOINT")

	if endpoint == "" {
		log.Fatalf("Agent endpoint not specified or configmap not found.")
	}

	pollingDuration, err := time.ParseDuration(os.Getenv("CW_POLLING"))
	if err != nil {
		log.Fatalf("Cannot determine cloudwatch polling interval: %s", err.Error())
	}

	filter := os.Getenv("CW_FILTER")

	eks, err := ekscloudwatch.New(endpoint, clusterNameOverride, pollingDuration)
	if err != nil {
		log.Fatalf("Could not initialize EKS CloudWatch client: %s", err)
	}

	err = eks.RunForever(filter)
	if err != nil {
		log.Fatalf("Error while running event CW client loop: %s", err)
	}
}
