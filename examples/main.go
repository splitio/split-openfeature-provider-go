// Example demonstrates all scenarios of the Split OpenFeature provider:
// setup (API key vs custom client), evaluations (bool/string/int/float/object),
// context levels, details (variant, FlagMetadata), tracking, and error handling.
//
// Run from the example directory:
//
//	go run .
//
// Uses localhost mode with split.yaml (no real API key required).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/splitio/go-client/v6/splitio/client"
	"github.com/splitio/go-client/v6/splitio/conf"
	splitProvider "github.com/splitio/split-openfeature-provider-go"
)

func main() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// Initialization: Split client (API key from env or localhost for demo)
	// -------------------------------------------------------------------------
	cfg := conf.Default()

	api_key := os.Getenv("SPLIT_API_KEY")

	factory, err := client.NewSplitFactory(api_key, cfg)
	if err != nil {
		log.Fatalf("create Split factory: %v", err)
	}
	splitClient := factory.Client()
	if err := splitClient.BlockUntilReady(10); err != nil {
		log.Fatalf("Split not ready: %v", err)
	}

	provider, err := splitProvider.NewProvider(splitClient)
	if err != nil {
		log.Fatalf("create provider: %v", err)
	}
	if err := openfeature.SetProviderAndWait(provider); err != nil {
		log.Fatalf("register provider: %v", err)
	}

	clientOF := openfeature.NewClient("example-app")
	fmt.Println("=== Split OpenFeature Provider - Full example ===\n")

	// Example user (targeting key). In production this would be the user/tenant ID.
	userID := "key"
	evalCtx := openfeature.NewEvaluationContext(userID, map[string]any{})

	// -------------------------------------------------------------------------
	// 1. Basic evaluations (all types)
	// -------------------------------------------------------------------------
	fmt.Println("--- 1. Basic evaluations ---")

	// Boolean
	on, err := clientOF.BooleanValue(ctx, "my_feature", false, evalCtx)
	if err != nil {
		log.Printf("  my_feature (bool): error %v", err)
	} else {
		fmt.Printf("  my_feature (bool): %v\n", on)
	}

	// String
	s, err := clientOF.StringValue(ctx, "some_other_feature", "default", evalCtx)
	if err != nil {
		log.Printf("  some_other_feature (string): error %v", err)
	} else {
		fmt.Printf("  some_other_feature (string): %q\n", s)
	}

	// Int
	n, err := clientOF.IntValue(ctx, "int_feature", 0, evalCtx)
	if err != nil {
		log.Printf("  int_feature (int): error %v", err)
	} else {
		fmt.Printf("  int_feature (int): %d\n", n)
	}

	// Float
	f, err := clientOF.FloatValue(ctx, "float_feature", 0, evalCtx)
	if err != nil {
		log.Printf("  float_feature (float): error %v", err)
	} else {
		fmt.Printf("  float_feature (float): %f\n", f)
	}

	// Object
	obj, err := clientOF.ObjectValue(ctx, "obj_feature", map[string]any{"variation": "default"}, evalCtx)
	if err != nil {
		log.Printf("  obj_feature (object): error %v, %v", err, obj)
	} else {
		fmt.Printf("  obj_feature (object): %v\n", obj)
	}

	// -------------------------------------------------------------------------
	// 2. Context at different levels (API, client, invocation)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 2. Context at different levels ---")

	// API (global) level
	globalCtx := openfeature.NewEvaluationContext("global-user", map[string]any{"source": "api"})
	openfeature.SetEvaluationContext(globalCtx)
	v, _ := clientOF.BooleanValue(ctx, "my_feature", false, openfeature.EvaluationContext{})
	fmt.Printf("  With global context (global-user): my_feature = %v\n", v)

	// Client level
	clientCtx := openfeature.NewEvaluationContext("client-user", map[string]any{"source": "client"})
	clientOF.SetEvaluationContext(clientCtx)
	v, _ = clientOF.BooleanValue(ctx, "my_feature", false, openfeature.EvaluationContext{})
	fmt.Printf("  With client context (client-user): my_feature = %v\n", v)

	// Invocation level (overrides)
	invokeCtx := openfeature.NewEvaluationContext(userID, map[string]any{"source": "invocation"})
	v, _ = clientOF.BooleanValue(ctx, "my_feature", false, invokeCtx)
	fmt.Printf("  With invocation context (%s): my_feature = %v\n", userID, v)

	// Restore example context for following sections
	clientOF.SetEvaluationContext(evalCtx)

	// -------------------------------------------------------------------------
	// 3. Evaluation with details (variant, reason, FlagMetadata["config"])
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 3. Evaluation with details ---")

	details, err := clientOF.BooleanValueDetails(ctx, "obj_feature", false, evalCtx)
	if err != nil {
		log.Printf("  BooleanValueDetails: %v", err)
	} else {
		fmt.Printf("  FlagKey: %s\n", details.FlagKey)
		fmt.Printf("  Value: %v\n", details.Value)
		fmt.Printf("  Variant: %s\n", details.Variant)
		fmt.Printf("  Reason: %s\n", details.Reason)
		if details.FlagMetadata != nil {
			if config, ok := details.FlagMetadata["config"].(string); ok && config != "" {
				fmt.Printf("  FlagMetadata[\"config\"]: %s\n", config)
			}
		}
	}

	strDetails, err := clientOF.StringValueDetails(ctx, "some_other_feature", "default", evalCtx)
	if err != nil {
		log.Printf("  StringValueDetails: %v", err)
	} else {
		fmt.Printf("  some_other_feature -> variant=%q reason=%s\n", strDetails.Variant, strDetails.Reason)
	}

	// -------------------------------------------------------------------------
	// 4. Tracking (events to Split)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 4. Tracking ---")

	trackCtx := openfeature.NewEvaluationContext("user-123", map[string]any{"trafficType": "user"})
	detailsTrack := openfeature.NewTrackingEventDetails(99.99).Add("plan", "pro").Add("currency", "USD")
	clientOF.Track(ctx, "checkout.completed", trackCtx, detailsTrack)
	fmt.Println("  Track('checkout.completed') sent (trafficType=user, value=99.99, plan=pro, currency=USD)")

	// -------------------------------------------------------------------------
	// 5. Error handling (missing targeting key, flag not found, parse error)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 5. Error handling ---")

	// Missing targeting key
	_, err = clientOF.BooleanValue(ctx, "my_feature", false, openfeature.EvaluationContext{})
	if err != nil {
		fmt.Printf("  Missing targeting key: %v\n", err)
	}

	// Non-existent flag (control / default)
	_, err = clientOF.BooleanValue(ctx, "non-existent-flag", true, evalCtx)
	if err != nil {
		fmt.Printf("  Non-existent flag: %v\n", err)
	} else {
		fmt.Println("  Non-existent flag: default value (true) returned")
	}

	// Parse error: request boolean for a treatment that is an object
	_, err = clientOF.BooleanValue(ctx, "obj_feature", false, evalCtx)
	if err != nil {
		fmt.Printf("  Parse error (object as bool): %v\n", err)
	}

	// -------------------------------------------------------------------------
	// 6. Realistic flow: service simulation (checkout per user)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- 6. Realistic flow (checkout per user) ---")

	users := []struct {
		id   string
		name string
	}{
		{"key", "User with key (split.yaml)"},
		{"randomKey", "User randomKey"},
		{"guest-1", "Guest"},
	}

	for _, u := range users {
		userCtx := openfeature.NewEvaluationContext(u.id, map[string]any{"trafficType": "user"})
		// Feature: show premium UI?
		premium, _ := clientOF.BooleanValue(ctx, "my_feature", false, userCtx)
		// Feature: text/state of another feature
		other, _ := clientOF.StringValue(ctx, "some_other_feature", "off", userCtx)
		fmt.Printf("  %s: premium=%v, some_other_feature=%q\n", u.name, premium, other)
		// Simulate page view tracking
		clientOF.Track(ctx, "page.view", userCtx, openfeature.NewTrackingEventDetails(0))
	}

	fmt.Println("\n=== End of example ===")
	os.Exit(0)
}
