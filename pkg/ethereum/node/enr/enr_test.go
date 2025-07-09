package enr

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		enr      string
		expected func(t *testing.T, result *ENR)
		wantErr  bool
	}{
		{
			name: "Valid ENR with IPv4 and TCP/UDP ports",
			enr:  "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI", result.Enr)
				assert.NotNil(t, result.Signature)
				assert.NotNil(t, result.Seq)
				assert.NotNil(t, result.ID)
				assert.NotNil(t, result.NodeID)
				assert.NotNil(t, result.Secp256k1)
				assert.NotNil(t, result.IP4)
				assert.Nil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.Equal(t, "v4", *result.ID)
				assert.Equal(t, "167.71.14.109", *result.IP4)
				assert.Equal(t, uint32(9010), *result.UDP4)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with ETH2 fields",
			enr:  "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.ETH2)
				assert.NotNil(t, result.Attnets)
				assert.NotNil(t, result.IP4)
				assert.Equal(t, "167.71.14.109", *result.IP4)
				assert.Equal(t, uint32(9000), *result.TCP4)
				assert.Equal(t, uint32(9000), *result.UDP4)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with client info",
			enr:  "enr:-Om4QEXRyeXzy_ck4VlL9DGnngByWxFESPO1-PFxyiFywmgELVAjJtUmgeAjdFefFN-ORW-pluJEyjNMKdz3ut8vWiwBh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcITRJl9qhHF1aWOCIymJc2VjcDI1NmsxoQI6n1m1LJzUDZKJSEQyRlOzLc9xVyCwVz3mLZLmxij-C4hzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.NodeID)
				assert.NotNil(t, result.PeerID)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with sync committees",
			enr:  "enr:-Oi4QFDm6C0HGsMNqhkHHJ_WeMp23T_Sprdk5CDceRNqXngif1WGfYJBEbTsX72k8Zc9mm5lAXk4GyQkE6HjJi-tdVsBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKpA6S-EcXVpY4IjKYlzZWNwMjU2azGhA6wjHpjVDsRKYzNF7hnqIxBaAYV7JbDWuhP9KA73evV4iHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.Syncnets)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with lighthouse client",
			enr:  "enr:-PO4QGYLgcLYl9DA4MK-hsrhV4FGQwIHEyORJKnEJ0JkQja5DTWXNC5xqZ0OI_NZJsIu7eaf2GxqXPC7f3DVlIOa30ADh2F0dG5ldHOIAAAGAAAAAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEpFrLSoNuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEC-HAEr6PikSNtSPQj7LoDBjzA4lRhjKXzLZMkfPa6c1CIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with prysm client",
			enr:  "enr:-PK4QMPyeMY235Ub_ZlqKl334zOs4sq8LtvswOzNQ3Bi_XIHQ2o6JNMG_Cv1Bv1BN4LI1pm2w_9Ehm1AmLwpbaSDKIsDh2F0dG5ldHOIAAAAAAAAwACDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISnYyFCg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQNVkFcMN73uXQ6WjNFbEWoNOYBJTX1oR5Ql5j83ioqUPohzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with different fork digest",
			enr:  "enr:-PO4QABi6GKdJwoNAlzRo7fXIHvQRzm5gv7OT6Om0jleW4TQOrHEtjcZZ2hZmYXvm9fKqLfWJ1w5aezuzQ_QpFAfh3YDh2F0dG5ldHOIAAAAAAADAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEn99xd4NuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEDzVa77_o452OzzqylcK2mA0DREidLotbGonvz3nogDS-Ic3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with different attestation subnets",
			enr:  "enr:-PK4QL7rCat3LnKlWYzkPvjeJm4Cbov42etNl6wQC4LW3aycPGk9OQch3moDuNrxVM0EdpttisPnNdJ6LDxftvs3WnMDh2F0dG5ldHOIAAAAAAAAADCDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISPxltZg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQKgdoc_mP2jR1--vOcnqCMebmsoMprQT6lm6ErkeicvSYhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
				assert.NotNil(t, result.Attnets)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with different fork digest and attestation subnets",
			enr:  "enr:-PO4QIGGOCV_6tCWfAzToLRyOPoFcLjqJ5YRAAvmemsj_omfQ0hO3LbbMBhKVbRWfBv99ePO3BnjNii-5CGHoKOkRNcDh2F0dG5ldHOIAAAAAABgAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEGJBl4INuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEDfr-2_YzcYmkKyLrWj6eqXFAi-0h9CPOu5iK4jkAzCDSIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
				assert.NotNil(t, result.Attnets)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with different fork digest and sync committees",
			enr:  "enr:-PK4QN_SGnW6OwNBTlSDUrFoeV4D2E_QM8-jv2cU8gfSwmFKAohShGYHccV8AEFXHxL-ogkqPN-ChTE0kM69kO94dEwDh2F0dG5ldHOIAAAAAAAAgAGDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcIShI3tlg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQI29da9ro2L6y2D-w3SdwOXrluU8FpXw5k_eskVN0cFgIhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
				assert.NotNil(t, result.Syncnets)
			},
			wantErr: false,
		},
		{
			name: "Valid ENR with different fork digest and attestation subnet configuration",
			enr:  "enr:-PO4QGb7ebw8vRA7S1kqoiRYZzms0JAOvGg8CCdcutk2tQj3GT0dMWlgR6_fzo1-o3C3NAqVb8YOzYT1cjTMK_gwWg0Dh2F0dG5ldHOIAAAAAAAADACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEgMfODINuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaED3Z9tGtlnWG-YMz_Qof_ZdkaT78PhYMchThHOTr2TEQCIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
			expected: func(t *testing.T, result *ENR) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "v4", *result.ID)
				assert.NotNil(t, result.IP4)
				assert.NotNil(t, result.TCP4)
				assert.NotNil(t, result.UDP4)
				assert.NotNil(t, result.ETH2)
				assert.NotNil(t, result.Attnets)
			},
			wantErr: false,
		},
		{
			name:    "Invalid ENR - empty string",
			enr:     "",
			wantErr: true,
		},
		{
			name:    "Invalid ENR - malformed string",
			enr:     "invalid-enr-string",
			wantErr: true,
		},
		{
			name:    "Invalid ENR - missing prefix",
			enr:     "Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
			wantErr: true,
		},
		{
			name:    "Invalid ENR - corrupted base64",
			enr:     "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI!!!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.expected != nil {
					tt.expected(t, result)
				}
			}
		})
	}
}

func TestParseSignature(t *testing.T) {
	tests := []struct {
		name string
		enr  string
	}{
		{
			name: "Valid ENR with signature",
			enr:  "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
		},
		{
			name: "Different ENR with signature",
			enr:  "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Signature)
			assert.Greater(t, len(*result.Signature), 0)
		})
	}
}

func TestParseSeq(t *testing.T) {
	tests := []struct {
		name        string
		enr         string
		expectedSeq *uint64
	}{
		{
			name: "Valid ENR with sequence number",
			enr:  "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
		},
		{
			name: "Different ENR with sequence number",
			enr:  "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Seq)
			assert.IsType(t, uint64(0), *result.Seq)
		})
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name       string
		enr        string
		expectedID string
	}{
		{
			name:       "Valid ENR with v4 ID scheme",
			enr:        "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
			expectedID: "v4",
		},
		{
			name:       "Another valid ENR with v4 ID scheme",
			enr:        "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
			expectedID: "v4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.ID)
			assert.Equal(t, tt.expectedID, *result.ID)
		})
	}
}

func TestParseSecp256k1(t *testing.T) {
	tests := []struct {
		name string
		enr  string
	}{
		{
			name: "Valid ENR with secp256k1 key",
			enr:  "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
		},
		{
			name: "Different ENR with secp256k1 key",
			enr:  "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Secp256k1)
			assert.Greater(t, len(*result.Secp256k1), 0)
		})
	}
}

func TestParseIPAddresses(t *testing.T) {
	tests := []struct {
		name        string
		enr         string
		expectedIP4 *string
		expectedIP6 *string
	}{
		{
			name:        "Valid ENR with IPv4 address",
			enr:         "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
			expectedIP4: stringPtr("167.71.14.109"),
			expectedIP6: nil,
		},
		{
			name:        "Different ENR with IPv4 address",
			enr:         "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
			expectedIP4: stringPtr("167.71.14.109"),
			expectedIP6: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectedIP4 != nil {
				require.NotNil(t, result.IP4)
				assert.Equal(t, *tt.expectedIP4, *result.IP4)
			} else {
				assert.Nil(t, result.IP4)
			}

			if tt.expectedIP6 != nil {
				require.NotNil(t, result.IP6)
				assert.Equal(t, *tt.expectedIP6, *result.IP6)
			} else {
				assert.Nil(t, result.IP6)
			}
		})
	}
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name         string
		enr          string
		expectedTCP4 *uint32
		expectedTCP6 *uint32
		expectedUDP4 *uint32
		expectedUDP6 *uint32
	}{
		{
			name:         "Valid ENR with TCP/UDP ports",
			enr:          "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
			expectedTCP4: nil,
			expectedTCP6: nil,
			expectedUDP4: uint32Ptr(9010),
			expectedUDP6: nil,
		},
		{
			name:         "Different ENR with TCP/UDP ports",
			enr:          "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
			expectedTCP4: uint32Ptr(9000),
			expectedTCP6: nil,
			expectedUDP4: uint32Ptr(9000),
			expectedUDP6: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectedTCP4 != nil {
				require.NotNil(t, result.TCP4)
				assert.Equal(t, *tt.expectedTCP4, *result.TCP4)
			} else {
				assert.Nil(t, result.TCP4)
			}

			if tt.expectedTCP6 != nil {
				require.NotNil(t, result.TCP6)
				assert.Equal(t, *tt.expectedTCP6, *result.TCP6)
			} else {
				assert.Nil(t, result.TCP6)
			}

			if tt.expectedUDP4 != nil {
				require.NotNil(t, result.UDP4)
				assert.Equal(t, *tt.expectedUDP4, *result.UDP4)
			} else {
				assert.Nil(t, result.UDP4)
			}

			if tt.expectedUDP6 != nil {
				require.NotNil(t, result.UDP6)
				assert.Equal(t, *tt.expectedUDP6, *result.UDP6)
			} else {
				assert.Nil(t, result.UDP6)
			}
		})
	}
}

func TestParseETH2Fields(t *testing.T) {
	tests := []struct {
		name           string
		enr            string
		expectETH2     bool
		expectAttnets  bool
		expectSyncnets bool
		expectCGC      bool
	}{
		{
			name:           "ENR with ETH2 and attestation subnets",
			enr:            "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
			expectETH2:     true,
			expectAttnets:  true,
			expectSyncnets: false,
			expectCGC:      true,
		},
		{
			name:           "ENR with sync committees",
			enr:            "enr:-Oi4QFDm6C0HGsMNqhkHHJ_WeMp23T_Sprdk5CDceRNqXngif1WGfYJBEbTsX72k8Zc9mm5lAXk4GyQkE6HjJi-tdVsBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKpA6S-EcXVpY4IjKYlzZWNwMjU2azGhA6wjHpjVDsRKYzNF7hnqIxBaAYV7JbDWuhP9KA73evV4iHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
			expectETH2:     true,
			expectAttnets:  true,
			expectSyncnets: true,
			expectCGC:      true,
		},
		{
			name:           "Lighthouse ENR with ETH2 key",
			enr:            "enr:-PO4QGYLgcLYl9DA4MK-hsrhV4FGQwIHEyORJKnEJ0JkQja5DTWXNC5xqZ0OI_NZJsIu7eaf2GxqXPC7f3DVlIOa30ADh2F0dG5ldHOIAAAGAAAAAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEpFrLSoNuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEC-HAEr6PikSNtSPQj7LoDBjzA4lRhjKXzLZMkfPa6c1CIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
			expectETH2:     true,
			expectAttnets:  true,
			expectSyncnets: true,
			expectCGC:      true,
		},
		{
			name:           "ENR with attestation subnets",
			enr:            "enr:-PK4QL7rCat3LnKlWYzkPvjeJm4Cbov42etNl6wQC4LW3aycPGk9OQch3moDuNrxVM0EdpttisPnNdJ6LDxftvs3WnMDh2F0dG5ldHOIAAAAAAAAADCDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISPxltZg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQKgdoc_mP2jR1--vOcnqCMebmsoMprQT6lm6ErkeicvSYhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
			expectETH2:     true,
			expectAttnets:  true,
			expectSyncnets: true,
			expectCGC:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectETH2 {
				assert.NotNil(t, result.ETH2, "Expected ETH2 field to be present")
				assert.Greater(t, len(*result.ETH2), 0, "ETH2 field should not be empty")
			} else {
				assert.Nil(t, result.ETH2, "Expected ETH2 field to be nil")
			}

			if tt.expectAttnets {
				assert.NotNil(t, result.Attnets, "Expected Attnets field to be present")
				assert.Greater(t, len(*result.Attnets), 0, "Attnets field should not be empty")
			} else {
				assert.Nil(t, result.Attnets, "Expected Attnets field to be nil")
			}

			if tt.expectSyncnets {
				assert.NotNil(t, result.Syncnets, "Expected Syncnets field to be present")
				assert.Greater(t, len(*result.Syncnets), 0, "Syncnets field should not be empty")
			} else {
				assert.Nil(t, result.Syncnets, "Expected Syncnets field to be nil")
			}

			if tt.expectCGC {
				assert.NotNil(t, result.CGC, "Expected CGC field to be present")
				assert.Greater(t, len(*result.CGC), 0, "CGC field should not be empty")
			} else {
				assert.Nil(t, result.CGC, "Expected CGC field to be nil")
			}
		})
	}
}

func TestParseNodeIDAndPeerID(t *testing.T) {
	tests := []struct {
		name string
		enr  string
	}{
		{
			name: "Valid ENR with node ID and peer ID",
			enr:  "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
		},
		{
			name: "Different ENR with node ID and peer ID",
			enr:  "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.enr)
			require.NoError(t, err)
			require.NotNil(t, result)

			require.NotNil(t, result.NodeID)
			assert.Greater(t, len(*result.NodeID), 0, "NodeID should not be empty")

			require.NotNil(t, result.PeerID)
			assert.Greater(t, len(*result.PeerID), 0, "PeerID should not be empty")
		})
	}
}

func TestParseAllProvidedENRs(t *testing.T) {
	enrs := []string{
		"enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI",
		"enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo",
		"enr:-Om4QEXRyeXzy_ck4VlL9DGnngByWxFESPO1-PFxyiFywmgELVAjJtUmgeAjdFefFN-ORW-pluJEyjNMKdz3ut8vWiwBh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcITRJl9qhHF1aWOCIymJc2VjcDI1NmsxoQI6n1m1LJzUDZKJSEQyRlOzLc9xVyCwVz3mLZLmxij-C4hzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-Oi4QFDm6C0HGsMNqhkHHJ_WeMp23T_Sprdk5CDceRNqXngif1WGfYJBEbTsX72k8Zc9mm5lAXk4GyQkE6HjJi-tdVsBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKpA6S-EcXVpY4IjKYlzZWNwMjU2azGhA6wjHpjVDsRKYzNF7hnqIxBaAYV7JbDWuhP9KA73evV4iHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
		"enr:-Om4QDHRJodEgX4nciJunJtsWsib4RUHdaIY4sVCWt_fjEIzG73UiAv5XF8sHvJ-hu3JQ5zzH4uhY9wzybiVeXsKZ1IBh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcIRDzbAGhHF1aWOCIymJc2VjcDI1NmsxoQNDZdeNWzem1fUKWgvb21EAnB90al4MsQBFVBxRxYBoNYhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-Oi4QMfHqzVEzWu18qjTsCaLX661o3-bZ8lnSmt6X7_onqE8fvF7DYRY24AalZPYA9MkeVGF_Y7C3KckLvf75PGyI1MBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKUW9uaEcXVpY4IjKYlzZWNwMjU2azGhApP2I3-SbhVdfMujQ9Ed8komght7kC9TpyU9H1lwMvHeiHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
		"enr:-Om4QBUbJIrpLLKdbgYEC9RkjdQXWaoJb36-YgI5Ct-JcF0ZEjyeWxvL3XqYtUtIbD4wLQ14m503FwG3lYKbkBVufxIBh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISkWuZzhHF1aWOCIymJc2VjcDI1NmsxoQLoNsikkJdbllw3ZsWTTlUNvSPovv5oCazO1Bf5yWflQ4hzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-Oi4QGKNtU_vwamGhnuzkr1WBwpgBoCPf2Eo6uwATezH5OxwEwnugN1EHD4QKbcYs0epdnIWDU92PFXu-CnZEzRTMrsBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhC5llUaEcXVpY4IjKYlzZWNwMjU2azGhA11dnrjCPJFjsyHhXRrgV7wJodrGFgiSGb4GCLWy8UkRiHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
		"enr:-Om4QM0OiRTHrxelkb3BcC95TOrxQDXkqIBIlSrQ6DowGtGldUqxavi5uWYXmzkQ-V-wlFybnd6DtK35VbcEA763mK8Bh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISJuMfYhHF1aWOCIymJc2VjcDI1NmsxoQLwF2EJgixHV_fsUNt65ogw9YSRIa8q1qwdI-2iON5fCYhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-Oi4QJXbBcypC97oiYvqeymrRZn9NY8G2qN1Q59g3uGWy-6petmojPdSzY8Ehvv13roHWjcNwwbuobRzWzj3q6jjYCYBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhJK-Q0CEcXVpY4IjKYlzZWNwMjU2azGhAu85dMr46CmEYLBupeTs-KXsFCbo6qOTDIzOZnHgCkIpiHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
		"enr:-Om4QNRySifCAycRCzFU55-Ark4VfVK5ZioGEUs-NI63EnWkcIMgkTVV4dwshvUudLSoiGNGMbu5eyTc8QphNxPBN2UBh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISTtrhQhHF1aWOCIymJc2VjcDI1NmsxoQJx6jFyMAG93kn_FakF_b6RIRTfG7mspd_CLA_4GihxCIhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-Oi4QMtiv_PgsabA3mYY0J8ojTGMu5esZpZ3bWGG5yqLjWsGFwbX4XGwZHB8GmUyhPL9k5GMKDwNpPzIkizbH7rRpPABh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhLKAf9WEcXVpY4IjKYlzZWNwMjU2azGhA5dOEKcVRBod5iOi1VJ6ZFj4nlnpPEDAk9EcJavvLnX3iHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
		"enr:-Om4QFapxOcjac9poWW4oRLWYKOLs637cIkvsCB7wcSSFTiTQep2k0Cj0VQaxK6L9gRd-vxNd8ILhr2FY9YSPzi-1t8Bh2F0dG5ldHOIAAAAAAAAAACDY2djgYCGY2xpZW5014hHcmFuZGluZY0xLjEuMS01NTkyM2I5hGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISfWaHThHF1aWOCIymJc2VjcDI1NmsxoQP4Bsn8EPPD-xVaBWV4keT4GodWs2pkn1eKrRbepylk7ohzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-Oi4QCzSI8rVN8DU8j45y58rUPGS7beoY93MPFfQRFiMFGcbJ5wbhqx-7QFfIVJGMnIsQC_W2NboCTsLRaBa5YEpjsYBh2F0dG5ldHOIAAAAAAAAAACDY2djBIZjbGllbnTXiEdyYW5kaW5ljTEuMS4xLTU1OTIzYjmEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhIs7KJuEcXVpY4IjKYlzZWNwMjU2azGhAqUx7lrYtjPLL5wKpdtTG1o6fMxO7VfkGtNYm2fHiKRliHN5bmNuZXRzAIN0Y3CCIyiDdWRwgiMo",
		"enr:-PO4QGYLgcLYl9DA4MK-hsrhV4FGQwIHEyORJKnEJ0JkQja5DTWXNC5xqZ0OI_NZJsIu7eaf2GxqXPC7f3DVlIOa30ADh2F0dG5ldHOIAAAGAAAAAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEpFrLSoNuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEC-HAEr6PikSNtSPQj7LoDBjzA4lRhjKXzLZMkfPa6c1CIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
		"enr:-PK4QMPyeMY235Ub_ZlqKl334zOs4sq8LtvswOzNQ3Bi_XIHQ2o6JNMG_Cv1Bv1BN4LI1pm2w_9Ehm1AmLwpbaSDKIsDh2F0dG5ldHOIAAAAAAAAwACDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISnYyFCg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQNVkFcMN73uXQ6WjNFbEWoNOYBJTX1oR5Ql5j83ioqUPohzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-PO4QABi6GKdJwoNAlzRo7fXIHvQRzm5gv7OT6Om0jleW4TQOrHEtjcZZ2hZmYXvm9fKqLfWJ1w5aezuzQ_QpFAfh3YDh2F0dG5ldHOIAAAAAAADAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEn99xd4NuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEDzVa77_o452OzzqylcK2mA0DREidLotbGonvz3nogDS-Ic3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
		"enr:-PK4QL7rCat3LnKlWYzkPvjeJm4Cbov42etNl6wQC4LW3aycPGk9OQch3moDuNrxVM0EdpttisPnNdJ6LDxftvs3WnMDh2F0dG5ldHOIAAAAAAAAADCDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcISPxltZg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQKgdoc_mP2jR1--vOcnqCMebmsoMprQT6lm6ErkeicvSYhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-PO4QIGGOCV_6tCWfAzToLRyOPoFcLjqJ5YRAAvmemsj_omfQ0hO3LbbMBhKVbRWfBv99ePO3BnjNii-5CGHoKOkRNcDh2F0dG5ldHOIAAAAAABgAACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEGJBl4INuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaEDfr-2_YzcYmkKyLrWj6eqXFAi-0h9CPOu5iK4jkAzCDSIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
		"enr:-PK4QN_SGnW6OwNBTlSDUrFoeV4D2E_QM8-jv2cU8gfSwmFKAohShGYHccV8AEFXHxL-ogkqPN-ChTE0kM69kO94dEwDh2F0dG5ldHOIAAAAAAAAgAGDY2djBIZjbGllbnTYikxpZ2h0aG91c2WMNy4xLjAtYmV0YS4whGV0aDKQgQATGnCTdUQAAQAAAAAAAIJpZIJ2NIJpcIShI3tlg25mZIQ2n4n3hHF1aWOCIymJc2VjcDI1NmsxoQI29da9ro2L6y2D-w3SdwOXrluU8FpXw5k_eskVN0cFgIhzeW5jbmV0cwCDdGNwgiMog3VkcIIjKA",
		"enr:-PO4QGb7ebw8vRA7S1kqoiRYZzms0JAOvGg8CCdcutk2tQj3GT0dMWlgR6_fzo1-o3C3NAqVb8YOzYT1cjTMK_gwWg0Dh2F0dG5ldHOIAAAAAAAADACDY2djgYCGY2xpZW502IpMaWdodGhvdXNljDcuMS4wLWJldGEuMIRldGgykIEAExpwk3VEAAEAAAAAAACCaWSCdjSCaXCEgMfODINuZmSENp-J94RxdWljgiMpiXNlY3AyNTZrMaED3Z9tGtlnWG-YMz_Qof_ZdkaT78PhYMchThHOTr2TEQCIc3luY25ldHMAg3RjcIIjKIN1ZHCCIyg",
	}

	for i, enr := range enrs {
		t.Run(fmt.Sprintf("ENR_%d", i+1), func(t *testing.T) {
			result, err := Parse(enr)
			require.NoError(t, err, "Failed to parse ENR %d: %s", i+1, enr)
			require.NotNil(t, result, "Result should not be nil for ENR %d", i+1)

			assert.Equal(t, enr, result.Enr, "ENR string should match input")
			assert.NotNil(t, result.Signature, "Signature should not be nil")
			assert.NotNil(t, result.Seq, "Seq should not be nil")
			assert.NotNil(t, result.ID, "ID should not be nil")
			assert.Equal(t, "v4", *result.ID, "ID should be v4")
			assert.NotNil(t, result.NodeID, "NodeID should not be nil")
			assert.NotNil(t, result.Secp256k1, "Secp256k1 should not be nil")
			assert.NotNil(t, result.IP4, "IP4 should not be nil")
			assert.NotNil(t, result.UDP4, "UDP4 should not be nil")
		})
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("Parse with nil input", func(t *testing.T) {
		result, err := Parse("")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Parse with whitespace", func(t *testing.T) {
		result, err := Parse("  \t\n  ")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Parse with random string", func(t *testing.T) {
		result, err := Parse("random-string-that-is-not-enr")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Parse with partial ENR", func(t *testing.T) {
		result, err := Parse("enr:")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Parse with truncated ENR", func(t *testing.T) {
		result, err := Parse("enr:-Iq4QCxbKw")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestParseFieldsWithMissingData(t *testing.T) {
	validENR := "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI"

	result, err := Parse(validENR)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Nil(t, result.IP6, "IP6 should be nil when not present")
	assert.Nil(t, result.TCP6, "TCP6 should be nil when not present")
	assert.Nil(t, result.UDP6, "UDP6 should be nil when not present")
	assert.Nil(t, result.CGC, "CGC should be nil when not present")
}

func stringPtr(s string) *string {
	return &s
}

func uint32Ptr(u uint32) *uint32 {
	return &u
}

func TestParseIP6Logic(t *testing.T) {
	t.Run("Test parseIP6 logic beyond Load() call", func(t *testing.T) {
		// Test with one of the provided ENRs to ensure the full parseIP6 logic is tested
		enrStr := "enr:-Iq4QCxbKw-XHdkvUcbd5-bJ8vEtyJr5jD3sg3XCwnkWXWwOEcuWWTrev8TnIcSsatTVd2LseQy1wH8u97vPGlxismiGAZerck1AgmlkgnY0gmlwhKdHDm2Jc2VjcDI1NmsxoQJJ3h8aUO3GJHv-bdvHtsQZ2OEisutelYfGjXO4lSg8BYN1ZHCCIzI"

		parsed, err := Parse(enrStr)
		require.NoError(t, err)
		require.NotNil(t, parsed)

		// This ENR should not have IPv6, so parseIP6 should return nil
		// This tests the err != nil path in parseIP6
		assert.Nil(t, parsed.IP6, "IPv6 should be nil for ENR without IPv6 field")

		// Test the parseIP6 function directly with the node
		node, err := enode.Parse(enode.ValidSchemes, enrStr)
		require.NoError(t, err)

		// Call parseIP6 directly to test the logic
		ip6Result := parseIP6(node)
		assert.Nil(t, ip6Result, "parseIP6 should return nil when no IPv6 field is present")
	})
}

func TestParseTCP6AndUDP6Logic(t *testing.T) {
	t.Run("Test parseTCP6 and parseUDP6 logic beyond Load() call", func(t *testing.T) {
		// Test with one of the provided ENRs
		enrStr := "enr:-MG4QGk5z8hpTrGM3uosvLuGmdL381IMXvmeBJBRxJUreV_cemmE-cJ6ftJRggPjM_tX6uhSEsO3mbqYpaSVTx4aYdYHh2F0dG5ldHOIAAAAAIABAACDY2djgYCEZXRoMpCBABMacJN1RAABAAAAAAAAgmlkgnY0gmlwhKdHDm2DbmZkhDafifeJc2VjcDI1NmsxoQN2BhqrvYI0XsXGaCnPcgLDwrwIL_szGrhtPGtb9_-AeYN0Y3CCIyiDdWRwgiMo"

		parsed, err := Parse(enrStr)
		require.NoError(t, err)
		require.NotNil(t, parsed)

		// This ENR should not have TCP6/UDP6 fields
		assert.Nil(t, parsed.TCP6, "TCP6 should be nil for ENR without TCP6 field")
		assert.Nil(t, parsed.UDP6, "UDP6 should be nil for ENR without UDP6 field")

		// Test the functions directly
		node, err := enode.Parse(enode.ValidSchemes, enrStr)
		require.NoError(t, err)

		// Call parseTCP6 and parseUDP6 directly to test the logic
		tcp6Result := parseTCP6(node)
		udp6Result := parseUDP6(node)

		assert.Nil(t, tcp6Result, "parseTCP6 should return nil when no TCP6 field is present")
		assert.Nil(t, udp6Result, "parseUDP6 should return nil when no UDP6 field is present")
	})
}
