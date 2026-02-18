// Package usdc provides exact-precision USDC amount handling using integer arithmetic.
// All financial amounts are stored as MicroUSDC (1 = 0.000001 USDC, i.e. $1.00 = 1_000_000).
package usdc

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// MicroUSDC represents a USDC amount in atomic units (1 = 0.000001 USDC).
// $1.00 = 1_000_000 microUSDC. $0.001 = 1_000 microUSDC.
type MicroUSDC int64

// Scale is the number of decimal places in MicroUSDC (10^6).
const Scale = 1_000_000

// FromFloat converts a human-readable float (e.g. 0.001) to MicroUSDC.
// Uses math.Round to avoid float truncation.
func FromFloat(f float64) MicroUSDC {
	return MicroUSDC(math.Round(f * Scale))
}

// Float returns the human-readable float64 value.
func (m MicroUSDC) Float() float64 {
	return float64(m) / Scale
}

// String returns a human-readable string with minimum 2 decimal places,
// trailing zeros trimmed beyond that.
// Examples: 1000000 → "1.00", 1000 → "0.001", 1250000 → "1.25", 100 → "0.0001"
func (m MicroUSDC) String() string {
	negative := m < 0
	v := int64(m)
	if negative {
		v = -v
	}

	whole := v / Scale
	frac := v % Scale

	// Format with 6 decimal places
	s := fmt.Sprintf("%d.%06d", whole, frac)

	// Trim trailing zeros but keep minimum 2 decimal places
	// Find the decimal point
	dotIdx := strings.IndexByte(s, '.')
	// Find last non-zero position after min 2 decimals
	minKeep := dotIdx + 3 // at least ".XX"
	lastNonZero := len(s) - 1
	for lastNonZero > minKeep-1 && s[lastNonZero] == '0' {
		lastNonZero--
	}
	s = s[:lastNonZero+1]

	if negative {
		return "-" + s
	}
	return s
}

// MarshalJSON outputs the raw integer as a JSON string: "1250000".
func (m MicroUSDC) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatInt(int64(m), 10) + `"`), nil
}

// UnmarshalJSON parses from a JSON string ("1250000") or number (1250000).
func (m *MicroUSDC) UnmarshalJSON(data []byte) error {
	s := string(data)

	// Handle quoted string: "1250000"
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("usdc: cannot parse %q as MicroUSDC: %w", string(data), err)
	}
	*m = MicroUSDC(v)
	return nil
}

// ToBigInt converts to *big.Int for blockchain operations.
// For chains where USDC decimals match the MicroUSDC scale (6), this is
// a direct conversion. For chains with different decimals, applies
// scaling: onChainUnits = microAmount * 10^(chainDecimals - 6).
func (m MicroUSDC) ToBigInt(chain string) *big.Int {
	decimals, ok := ChainDecimals[chain]
	if !ok {
		decimals = 6 // default to 6
	}

	result := big.NewInt(int64(m))
	if decimals > 6 {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals-6)), nil)
		result.Mul(result, scale)
	} else if decimals < 6 {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(6-decimals)), nil)
		result.Div(result, scale)
	}
	return result
}

// FromBigInt converts on-chain atomic units to MicroUSDC.
// Reverse of ToBigInt — scales based on chain decimals.
func FromBigInt(b *big.Int, chain string) MicroUSDC {
	decimals, ok := ChainDecimals[chain]
	if !ok {
		decimals = 6
	}

	result := new(big.Int).Set(b)
	if decimals > 6 {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals-6)), nil)
		result.Div(result, scale)
	} else if decimals < 6 {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(6-decimals)), nil)
		result.Mul(result, scale)
	}
	return MicroUSDC(result.Int64())
}

// ScaleForChain returns 10^ChainDecimals[chain].
func ScaleForChain(chain string) int64 {
	decimals, ok := ChainDecimals[chain]
	if !ok {
		decimals = 6
	}
	result := int64(1)
	for i := 0; i < decimals; i++ {
		result *= 10
	}
	return result
}
