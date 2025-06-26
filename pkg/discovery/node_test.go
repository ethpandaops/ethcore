package discovery

import (
	"testing"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestENRToEnode(t *testing.T) {
	tests := []struct {
		name    string
		enr     string
		wantErr bool
		errMsg  string
		checks  func(t *testing.T, node interface{})
	}{
		{
			name:    "valid ENR with TCP and UDP",
			enr:     "enr:-Ly4QFjn4bBBBIwydCq0eGNediB2aRyqYxqjhdWIy_8z-0OaNBhqVSmcwBGvxuw1MZ3wZnIxmWNDiLkkHIz6SDsP_LYIh2F0dG5ldHOIAAAAAACAAQCEZXRoMpBdbnkQYAAAOP__________gmlkgnY0gmlwhKwQJBKJc2VjcDI1NmsxoQIXm3rBsMBRPO8dNPbtJOgvBqyHme8kInz4J1g63ZrYaIhzeW5jbmV0cw-DdGNwgiMog3VkcIIjKA",
			wantErr: false,
			checks: func(t *testing.T, n interface{}) {
				t.Helper()
				node, ok := n.(*enode.Node)
				require.True(t, ok, "expected *enode.Node type")

				assert.NotNil(t, node)
				assert.Equal(t, "172.16.36.18", node.IP().String())
				assert.Equal(t, 9000, node.TCP())
				assert.Equal(t, 9000, node.UDP())
				assert.NotEmpty(t, node.ID())
			},
		},
		{
			name:    "valid ENR with localhost IP",
			enr:     "enr:-IS4QHCYrYZbAKWCBRlAy5zzaDZXJBGkcnh4MHcBFZntXNFrdvJjX04jRzjzCBOonrkTfj499SZuOh8R33Ls8RRcy5wBgmlkgnY0gmlwhH8AAAGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOIN1ZHCCdl8",
			wantErr: false,
			checks: func(t *testing.T, n interface{}) {
				t.Helper()
				node, ok := n.(*enode.Node)
				require.True(t, ok, "expected *enode.Node type")

				assert.NotNil(t, node)
				assert.Equal(t, "127.0.0.1", node.IP().String())
				assert.NotEmpty(t, node.ID())
			},
		},
		{
			name:    "valid enode URL",
			enr:     "enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@127.0.0.1:30303",
			wantErr: false,
			checks: func(t *testing.T, n interface{}) {
				t.Helper()
				node, ok := n.(*enode.Node)
				require.True(t, ok, "expected *enode.Node type")

				assert.NotNil(t, node)
				assert.Equal(t, "127.0.0.1", node.IP().String())
				assert.Equal(t, 30303, node.TCP())
				assert.Equal(t, 30303, node.UDP())
				assert.NotNil(t, node.Pubkey())
			},
		},
		{
			name:    "invalid ENR - malformed string",
			enr:     "enr:-invalid",
			wantErr: true,
			errMsg:  "failed to parse ENR",
		},
		{
			name:    "empty ENR string",
			enr:     "",
			wantErr: true,
			errMsg:  "failed to parse ENR",
		},
		{
			name:    "invalid scheme",
			enr:     "http://example.com",
			wantErr: true,
			errMsg:  "failed to parse ENR",
		},
		{
			name:    "invalid enode - missing port",
			enr:     "enode://a448f24c6acb5b78d7b09277818d99934ca11172968cb89d3c6ab353eb7e8fe25f7090bfb3342807a983614e603f519b92f33c5d980c96e08818adf0b8bed0f@127.0.0.1",
			wantErr: true,
			errMsg:  "failed to parse ENR",
		},
		{
			name:    "invalid enode - bad public key",
			enr:     "enode://invalid@127.0.0.1:30303",
			wantErr: true,
			errMsg:  "failed to parse ENR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ENRToEnode(tt.enr)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}

			require.NoError(t, err)
			if tt.checks != nil {
				tt.checks(t, node)
			}
		})
	}
}
