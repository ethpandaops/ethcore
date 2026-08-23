package ethereum_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// startStubBeacon stands up a minimal fake beacon REST API implementing
// just the endpoints BeaconNode and its metadata service call during
// startup. When identityFails is true, /eth/v1/node/identity always
// errors, modelling an upstream beacon whose identity endpoint is down --
// the trigger for ETHEREUM-06.
func startStubBeacon(t *testing.T, identityFails bool) string {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/eth/v1/node/syncing", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"head_slot": "100", "sync_distance": "0",
				"is_syncing": false, "is_optimistic": false,
			},
		})
	})

	mux.HandleFunc("/eth/v1/node/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"data": map[string]any{"version": "stub-beacon/1.0.0"}})
	})

	mux.HandleFunc("/eth/v1/config/spec", func(w http.ResponseWriter, r *http.Request) {
		// A real mainnet deposit contract, so DeriveNetwork resolves
		// cleanly and this test isolates the identity failure from
		// ETHEREUM-02's unresolvable-spec scenario.
		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"CONFIG_NAME":              "mainnet",
				"DEPOSIT_CHAIN_ID":         "1",
				"DEPOSIT_CONTRACT_ADDRESS": "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				"SECONDS_PER_SLOT":         "12",
				"SLOTS_PER_EPOCH":          "32",
			},
		})
	})

	mux.HandleFunc("/eth/v1/beacon/genesis", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"genesis_time":            "1606824023",
				"genesis_validators_root": "0x4e0c81aae871b0dccc5cae2e0e6c6f0071a79fbb8fd6e352c44b93a5c0fcff89",
				"genesis_fork_version":    "0x00000000",
			},
		})
	})

	mux.HandleFunc("/eth/v1/node/identity", func(w http.ResponseWriter, r *http.Request) {
		if identityFails {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"message":"identity unavailable"}`))

			return
		}

		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"peer_id":             "16Uiu2HAkwZbSganfHQjrhFDUgHufwUt2ZbGZWn9rV5rSSVAhVwTM",
				"enr":                 "enr:-Iq4QOevBudFrDwYocBEMU2ZBOL2FGSXxfMR9AsvggPRWtpAjcwyfd15SbtwuMK-6yF9Sqx87awq_ip5cchxwtLbbwWGAX53x8JgmpZk0NGnAQIeQ0Yg9SJqTGKZWjSElhLuYUX02fgQjA",
				"p2p_addresses":       []string{},
				"discovery_addresses": []string{},
				"metadata": map[string]any{
					"seq_number": "1",
					"attnets":    "0x0000000000000000",
					"syncnets":   "0x00",
				},
			},
		})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(mux)
	srv.Listener.Close() //nolint:errcheck
	srv.Listener = ln
	srv.Start()

	t.Cleanup(srv.Close)

	return srv.URL
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

// TestBeaconNode_IdentityFailureDoesNotHangStart guards against
// ETHEREUM-06: a beacon that is healthy and has a resolvable spec, but
// whose identity endpoint fails, used to leave serviceReady permanently
// unclosed. BeaconNode.Start would then hang until the caller's own ctx
// ran out, however long that was, instead of returning promptly with an
// error describing what actually went wrong.
func TestBeaconNode_IdentityFailureDoesNotHangStart(t *testing.T) {
	beaconURL := startStubBeacon(t, true)

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	opts := &ethereum.Options{Options: beacon.DefaultOptions()}
	opts.DisablePrometheusMetrics()
	opts.HealthCheck.Interval.Duration = 200 * time.Millisecond
	opts.HealthCheck.SuccessfulResponses = 1

	node, err := ethereum.NewBeaconNode(log, "test-identity-failure", &ethereum.Config{
		BeaconNodeAddress: beaconURL,
	}, opts)
	require.NoError(t, err)

	// Generous relative to the health check interval above, but a small
	// fraction of how long this call used to hang for (until context
	// cancellation, whatever that was set to by the caller -- potentially
	// forever). A fast, well-understood failure is what this test proves.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	startErr := node.Start(ctx)
	elapsed := time.Since(start)

	require.Error(t, startErr)
	require.NotErrorIs(t, startErr, context.DeadlineExceeded,
		"Start should fail because of the identity error, not because our ctx ran out")
	require.Less(t, elapsed, 5*time.Second, "Start took too long to report the identity failure")
}
