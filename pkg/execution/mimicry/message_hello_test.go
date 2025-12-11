package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelloCode(t *testing.T) {
	h := &Hello{}
	assert.Equal(t, HelloCode, h.Code())
	assert.Equal(t, 0x00, h.Code())
}

func TestHelloReqID(t *testing.T) {
	h := &Hello{}
	assert.Equal(t, uint64(0), h.ReqID())
}

func TestHelloValidate(t *testing.T) {
	tests := []struct {
		name    string
		hello   *Hello
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid hello with ETH 68",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 68},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: false,
		},
		{
			name: "valid hello with ETH 68 and 69",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 68},
					{Name: ETHCapName, Version: 69},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: false,
		},
		{
			name: "ETH 69 only without ETH 68 is invalid",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 69},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: true,
			errMsg:  "peer is using unsupported eth protocol version",
		},
		{
			name: "valid hello with multiple ETH versions",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 68},
					{Name: ETHCapName, Version: 69},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: false,
		},
		{
			name: "valid hello with mixed capabilities",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: "snap", Version: 1},
					{Name: ETHCapName, Version: 68},
					{Name: "les", Version: 4},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: false,
		},
		{
			name: "invalid P2P protocol version",
			hello: &Hello{
				Version: 4, // Less than minP2PProtocolVersion (5)
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 68},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: true,
			errMsg:  "peer is using unsupported p2p protocol version",
		},
		{
			name: "missing ETH capability",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: "snap", Version: 1},
					{Name: "les", Version: 4},
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: true,
			errMsg:  "peer does not support eth protocol",
		},
		{
			name: "no capabilities",
			hello: &Hello{
				Version:    P2PProtocolVersion,
				Name:       "test-client",
				Caps:       []p2p.Cap{},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: true,
			errMsg:  "peer does not support eth protocol",
		},
		{
			name: "unsupported ETH protocol version only",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 67}, // Below minETHProtocolVersion (68)
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: true,
			errMsg:  "peer is using unsupported eth protocol version",
		},
		{
			name: "only future unsupported ETH version",
			hello: &Hello{
				Version: P2PProtocolVersion,
				Name:    "test-client",
				Caps: []p2p.Cap{
					{Name: ETHCapName, Version: 70}, // Above maxETHProtocolVersion
				},
				ListenPort: 30303,
				ID:         make([]byte, 64),
			},
			wantErr: true,
			errMsg:  "peer is using unsupported eth protocol version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hello.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestHelloETHProtocolVersion(t *testing.T) {
	tests := []struct {
		name     string
		caps     []p2p.Cap
		expected uint
	}{
		{
			name: "ETH 68 only",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 68},
			},
			expected: 68,
		},
		{
			name: "ETH 69 only",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 69},
			},
			expected: 69,
		},
		{
			name: "ETH 68 and 69 returns highest supported",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 68},
				{Name: ETHCapName, Version: 69},
			},
			expected: 69,
		},
		{
			name: "ETH 70 unsupported returns max supported 69",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 68},
				{Name: ETHCapName, Version: 69},
				{Name: ETHCapName, Version: 70},
			},
			expected: 69,
		},
		{
			name: "only ETH 70 returns 0 since it exceeds max",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 70},
			},
			expected: 0,
		},
		{
			name:     "no ETH caps returns 0",
			caps:     []p2p.Cap{},
			expected: 0,
		},
		{
			name: "non-ETH caps only returns 0",
			caps: []p2p.Cap{
				{Name: "snap", Version: 1},
				{Name: "les", Version: 4},
			},
			expected: 0,
		},
		{
			name: "mixed caps returns correct ETH version",
			caps: []p2p.Cap{
				{Name: "snap", Version: 1},
				{Name: ETHCapName, Version: 68},
				{Name: "les", Version: 4},
			},
			expected: 68,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hello{Caps: tt.caps}
			result := h.ETHProtocolVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHelloETHCap(t *testing.T) {
	tests := []struct {
		name     string
		caps     []p2p.Cap
		expected *p2p.Cap
	}{
		{
			name: "returns ETH cap when present",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 68},
			},
			expected: &p2p.Cap{Name: ETHCapName, Version: 68},
		},
		{
			name: "returns first ETH cap when multiple present",
			caps: []p2p.Cap{
				{Name: ETHCapName, Version: 68},
				{Name: ETHCapName, Version: 69},
			},
			expected: &p2p.Cap{Name: ETHCapName, Version: 68},
		},
		{
			name: "returns nil when no ETH cap",
			caps: []p2p.Cap{
				{Name: "snap", Version: 1},
			},
			expected: nil,
		},
		{
			name:     "returns nil for empty caps",
			caps:     []p2p.Cap{},
			expected: nil,
		},
		{
			name: "finds ETH cap among mixed caps",
			caps: []p2p.Cap{
				{Name: "snap", Version: 1},
				{Name: ETHCapName, Version: 69},
				{Name: "les", Version: 4},
			},
			expected: &p2p.Cap{Name: ETHCapName, Version: 69},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hello{Caps: tt.caps}
			result := h.ETHCap()
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Version, result.Version)
			}
		})
	}
}

func TestSupportedEthCaps(t *testing.T) {
	caps := SupportedEthCaps()

	require.Len(t, caps, 2, "should support ETH 68 and 69")

	assert.Equal(t, ETHCapName, caps[0].Name)
	assert.Equal(t, uint(68), caps[0].Version)

	assert.Equal(t, ETHCapName, caps[1].Name)
	assert.Equal(t, uint(69), caps[1].Version)
}

func TestHelloRLPEncoding(t *testing.T) {
	original := &Hello{
		Version: P2PProtocolVersion,
		Name:    "test-client/v1.0.0",
		Caps: []p2p.Cap{
			{Name: ETHCapName, Version: 68},
			{Name: ETHCapName, Version: 69},
		},
		ListenPort: 30303,
		ID:         make([]byte, 64),
	}

	// Fill ID with recognizable data
	for i := range original.ID {
		original.ID[i] = byte(i)
	}

	// Encode
	encoded, err := rlp.EncodeToBytes(original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode
	decoded := new(Hello)
	err = rlp.DecodeBytes(encoded, decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.Version, decoded.Version)
	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.ListenPort, decoded.ListenPort)
	assert.Equal(t, original.ID, decoded.ID)
	require.Len(t, decoded.Caps, len(original.Caps))

	for i, cap := range original.Caps {
		assert.Equal(t, cap.Name, decoded.Caps[i].Name)
		assert.Equal(t, cap.Version, decoded.Caps[i].Version)
	}
}

func TestHelloRLPEncodingWithRest(t *testing.T) {
	// Test that the Rest field handles forward compatibility
	original := &Hello{
		Version: P2PProtocolVersion,
		Name:    "test-client",
		Caps: []p2p.Cap{
			{Name: ETHCapName, Version: 68},
		},
		ListenPort: 30303,
		ID:         make([]byte, 64),
		Rest:       []rlp.RawValue{[]byte{0x01, 0x02, 0x03}},
	}

	encoded, err := rlp.EncodeToBytes(original)
	require.NoError(t, err)

	decoded := new(Hello)
	err = rlp.DecodeBytes(encoded, decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Version, decoded.Version)
	assert.Equal(t, original.Name, decoded.Name)
}
