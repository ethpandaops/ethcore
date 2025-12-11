package mimicry

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNodeRecord(t *testing.T) {
	tests := []struct {
		name    string
		record  string
		wantErr bool
		checks  func(t *testing.T, record string)
	}{
		{
			name:    "valid enode URL",
			record:  "enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@127.0.0.1:30303",
			wantErr: false,
			checks: func(t *testing.T, record string) {
				t.Helper()
				node, err := parseNodeRecord(record)
				require.NoError(t, err)
				assert.Equal(t, "127.0.0.1", node.IP().String())
				assert.Equal(t, 30303, node.TCP())
				assert.NotNil(t, node.Pubkey())
			},
		},
		{
			name:    "valid enode URL with different port",
			record:  "enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@192.168.1.1:8545",
			wantErr: false,
			checks: func(t *testing.T, record string) {
				t.Helper()
				node, err := parseNodeRecord(record)
				require.NoError(t, err)
				assert.Equal(t, "192.168.1.1", node.IP().String())
				assert.Equal(t, 8545, node.TCP())
			},
		},
		{
			name:    "valid ENR string",
			record:  "enr:-IS4QHCYrYZbAKWCBRlAy5zzaDZXJBGkcnh4MHcBFZntXNFrdvJjX04jRzjzCBOonrkTfj499SZuOh8R33Ls8RRcy5wBgmlkgnY0gmlwhH8AAAGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOIN1ZHCCdl8",
			wantErr: false,
			checks: func(t *testing.T, record string) {
				t.Helper()
				node, err := parseNodeRecord(record)
				require.NoError(t, err)
				assert.NotNil(t, node)
				assert.NotEmpty(t, node.ID())
			},
		},
		{
			name:    "invalid enode - malformed",
			record:  "enode://invalid@127.0.0.1:30303",
			wantErr: true,
		},
		{
			name:    "invalid enode - missing port",
			record:  "enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@127.0.0.1",
			wantErr: true,
		},
		{
			name:    "invalid ENR - malformed",
			record:  "enr:-invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			record:  "",
			wantErr: true,
		},
		{
			name:    "random string",
			record:  "not-a-valid-record",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			record:  "http://example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := parseNodeRecord(tt.record)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, node)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, node)

			if tt.checks != nil {
				tt.checks(t, tt.record)
			}
		})
	}
}

func TestNew(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Suppress logs during tests

	tests := []struct {
		name    string
		record  string
		cname   string
		wantErr bool
		checks  func(t *testing.T, client *Client)
	}{
		{
			name:    "valid enode creates client",
			record:  "enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@127.0.0.1:30303",
			cname:   "test-client",
			wantErr: false,
			checks: func(t *testing.T, client *Client) {
				t.Helper()
				assert.NotNil(t, client.nodeRecord)
				assert.Equal(t, "test-client", client.name)
				assert.NotNil(t, client.broker)
				assert.NotNil(t, client.pooledTransactionsMap)
				assert.Empty(t, client.pooledTransactionsMap)
				assert.NotNil(t, client.log)
			},
		},
		{
			name:    "valid ENR creates client",
			record:  "enr:-IS4QHCYrYZbAKWCBRlAy5zzaDZXJBGkcnh4MHcBFZntXNFrdvJjX04jRzjzCBOonrkTfj499SZuOh8R33Ls8RRcy5wBgmlkgnY0gmlwhH8AAAGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOIN1ZHCCdl8",
			cname:   "enr-client",
			wantErr: false,
			checks: func(t *testing.T, client *Client) {
				t.Helper()
				assert.NotNil(t, client.nodeRecord)
				assert.Equal(t, "enr-client", client.name)
				assert.NotNil(t, client.broker)
			},
		},
		{
			name:    "empty name is allowed",
			record:  "enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@127.0.0.1:30303",
			cname:   "",
			wantErr: false,
			checks: func(t *testing.T, client *Client) {
				t.Helper()
				assert.Equal(t, "", client.name)
			},
		},
		{
			name:    "invalid record returns error",
			record:  "invalid-record",
			cname:   "test-client",
			wantErr: true,
		},
		{
			name:    "empty record returns error",
			record:  "",
			cname:   "test-client",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := New(ctx, log, tt.record, tt.cname)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, client)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)

			if tt.checks != nil {
				tt.checks(t, client)
			}
		})
	}
}

func TestClientConstants(t *testing.T) {
	assert.Equal(t, 0x10, RLPXOffset, "RLPXOffset should be 0x10 per RLPx spec")
}
