package discovery

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveDetailsFromNode(t *testing.T) {
	tests := []struct {
		name    string
		enr     string
		wantErr bool
		errMsg  string
		checks  func(t *testing.T, peer *ConnectablePeer)
	}{
		{
			name: "valid ENR with TCP and UDP",
			// This is a real ENR from the test output you provided
			enr:     "enr:-Ly4QFjn4bBBBIwydCq0eGNediB2aRyqYxqjhdWIy_8z-0OaNBhqVSmcwBGvxuw1MZ3wZnIxmWNDiLkkHIz6SDsP_LYIh2F0dG5ldHOIAAAAAACAAQCEZXRoMpBdbnkQYAAAOP__________gmlkgnY0gmlwhKwQJBKJc2VjcDI1NmsxoQIXm3rBsMBRPO8dNPbtJOgvBqyHme8kInz4J1g63ZrYaIhzeW5jbmV0cw-DdGNwgiMog3VkcIIjKA",
			wantErr: false,
			checks: func(t *testing.T, peer *ConnectablePeer) {
				t.Helper()

				assert.NotNil(t, peer)
				assert.NotNil(t, peer.Enode)
				assert.NotEmpty(t, peer.AddrInfo.ID)
				assert.NotEmpty(t, peer.AddrInfo.Addrs)

				// Check that we have both TCP and UDP addresses
				assert.Len(t, peer.AddrInfo.Addrs, 2)

				// Verify IP address
				assert.Equal(t, "172.16.36.18", peer.Enode.IP().String())

				// Verify ports
				assert.Equal(t, 9000, peer.Enode.TCP())
				assert.Equal(t, 9000, peer.Enode.UDP())
			},
		},
		{
			name: "valid ENR with IPv6",
			// Creating a test ENR with IPv6 - this is a constructed example
			enr:     "enr:-IS4QHCYrYZbAKWCBRlAy5zzaDZXJBGkcnh4MHcBFZntXNFrdvJjX04jRzjzCBOonrkTfj499SZuOh8R33Ls8RRcy5wBgmlkgnY0gmlwhH8AAAGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOIN1ZHCCdl8",
			wantErr: false,
			checks: func(t *testing.T, peer *ConnectablePeer) {
				t.Helper()

				assert.NotNil(t, peer)
				assert.NotNil(t, peer.Enode)
				assert.NotEmpty(t, peer.AddrInfo.ID)
				assert.NotEmpty(t, peer.AddrInfo.Addrs)

				// Should have at least one address
				assert.GreaterOrEqual(t, len(peer.AddrInfo.Addrs), 1)
			},
		},
		{
			name:    "invalid ENR string",
			enr:     "not-a-valid-enr",
			wantErr: true,
			errMsg:  "failed to parse",
		},
		{
			name:    "empty ENR string",
			enr:     "",
			wantErr: true,
			errMsg:  "failed to parse",
		},
		{
			name: "ENR without IP address",
			// This is a minimal ENR without IP/port information
			enr:     "enr:-HW4QBzimRxkmT18hMKaAL3IcZF1UcfTMPyi3Q1pxwZZbcZVRI8DC5infUAB_UauARLOJtYTxaagKoGmIjzQxO2qUygBgmlkgnY0iXNlY3AyNTZrMaEDymNMrg1JrLQB2KTGtv6MVbcNEVv0AHacwUAPMljNMTg",
			wantErr: true,
			errMsg:  "no IP address found in ENR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the ENR first
			node, err := enode.Parse(enode.ValidSchemes, tt.enr)
			if tt.enr == "" || tt.enr == "not-a-valid-enr" {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)

			// Test DeriveDetailsFromNode
			peer, err := DeriveDetailsFromNode(node)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, peer)

			if tt.checks != nil {
				tt.checks(t, peer)
			}
		})
	}
}

func TestDeriveDetailsFromNode_NilNode(t *testing.T) {
	// Test with nil node
	peer, err := DeriveDetailsFromNode(nil)
	require.Error(t, err)
	require.Nil(t, peer)
}

func TestDeriveDetailsFromNode_MultipleAddresses(t *testing.T) {
	// Test ENR with both TCP and UDP ports
	enr := "enr:-N24QDmMDgVMSgU4N21bA3XfeRywkdEUydsQE4ALXFtGFNKTAhqJCSjg3MtAOM7oT2Nxftaom_ZBBgxs_5YO-ASjqGYDh2F0dG5ldHOIAAAAAAAGAACGY2xpZW500YpMaWdodGhvdXNlhTcuMC4xhGV0aDKQXW55EGAAADj__________4JpZIJ2NIJpcISsECQRhHF1aWOCIymJc2VjcDI1NmsxoQJL6rLlF6F44evb6jDnEc57CQIaO58_PAdcarW7a-XJ54hzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA"

	node, err := enode.Parse(enode.ValidSchemes, enr)
	require.NoError(t, err)

	peer, err := DeriveDetailsFromNode(node)
	require.NoError(t, err)
	require.NotNil(t, peer)

	// Should have exactly 2 addresses (TCP and UDP)
	assert.Len(t, peer.AddrInfo.Addrs, 2)

	// Check that multiaddresses are properly formatted
	tcpFound := false
	udpFound := false
	for _, addr := range peer.AddrInfo.Addrs {
		addrStr := addr.String()
		assert.Contains(t, addrStr, "/ip4/")
		if strings.Contains(addrStr, "/tcp/") {
			tcpFound = true
		}
		if strings.Contains(addrStr, "/udp/") {
			udpFound = true
		}
	}
	assert.True(t, tcpFound, "Should have at least one TCP address")
	assert.True(t, udpFound, "Should have at least one UDP address")
}
