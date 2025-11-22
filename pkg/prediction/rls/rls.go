// Copyright The AIGW Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rls

const TPOT_COEFF_NUM = 2

// TpotRecursiveLeastSquares
type TpotRecursiveLeastSquares struct {
	theta  []float64
	P      [][]float64
	forget float64
}

// NewTpotRLS create RLS instance
func NewTpotRLS(forget float64) *TpotRecursiveLeastSquares {
	if forget <= 0 || forget > 1 {
		forget = 1.0
	}
	size := TPOT_COEFF_NUM + 1
	theta := make([]float64, size)
	P := make([][]float64, size)
	for i := 0; i < size; i++ {
		P[i] = make([]float64, size)
		P[i][i] = 1e6
	}
	return &TpotRecursiveLeastSquares{
		theta:  theta,
		P:      P,
		forget: forget,
	}
}

func (r *TpotRecursiveLeastSquares) Update(x []uint64, y float64) {
	if len(x) != TPOT_COEFF_NUM {
		return
	}

	// phi = [x0, x1, 1]
	// since the input x is limited, it's safe to convert uint64 to float64
	phi0 := float64(x[0])
	phi1 := float64(x[1])
	phi2 := 1.0

	P00 := r.P[0][0]
	P01 := r.P[0][1]
	P02 := r.P[0][2]
	P10 := r.P[1][0]
	P11 := r.P[1][1]
	P12 := r.P[1][2]
	P20 := r.P[2][0]
	P21 := r.P[2][1]
	P22 := r.P[2][2]

	// PHI = P * phi
	PHI0 := P00*phi0 + P01*phi1 + P02*phi2
	PHI1 := P10*phi0 + P11*phi1 + P12*phi2
	PHI2 := P20*phi0 + P21*phi1 + P22*phi2

	// den = forget + phiᵀ * PHI
	den := r.forget +
		phi0*PHI0 +
		phi1*PHI1 +
		phi2*PHI2
	invDen := 1.0 / den

	// K = PHI / den
	K0 := PHI0 * invDen
	K1 := PHI1 * invDen
	K2 := PHI2 * invDen

	// yPred = phiᵀ * theta
	yPred := phi0*r.theta[0] + phi1*r.theta[1] + phi2*r.theta[2]
	e := y - yPred

	// optimize theta
	r.theta[0] += K0 * e
	r.theta[1] += K1 * e
	r.theta[2] += K2 * e

	// update P, P = (P - K * PHIᵀ) / forget
	finv := 1.0 / r.forget

	r.P[0][0] = (P00 - K0*PHI0) * finv
	r.P[0][1] = (P01 - K0*PHI1) * finv
	r.P[0][2] = (P02 - K0*PHI2) * finv

	r.P[1][0] = (P10 - K1*PHI0) * finv
	r.P[1][1] = (P11 - K1*PHI1) * finv
	r.P[1][2] = (P12 - K1*PHI2) * finv

	r.P[2][0] = (P20 - K2*PHI0) * finv
	r.P[2][1] = (P21 - K2*PHI1) * finv
	r.P[2][2] = (P22 - K2*PHI2) * finv
}

// Predict
func (r *TpotRecursiveLeastSquares) Predict(x []uint64) float64 {
	if len(x) != TPOT_COEFF_NUM {
		return -1
	}
	size := TPOT_COEFF_NUM + 1
	phi := make([]uint64, size)
	copy(phi, x)
	phi[size-1] = 1
	y := float64(phi[0])*r.theta[0] + float64(phi[1])*r.theta[1] + float64(phi[2])*r.theta[2]
	return y
}

// Params return the current coefficients [a1..an, c]
func (r *TpotRecursiveLeastSquares) Params() []float64 {
	out := make([]float64, len(r.theta))
	copy(out, r.theta)
	return out
}

func (r *TpotRecursiveLeastSquares) Clone() *TpotRecursiveLeastSquares {
	size := TPOT_COEFF_NUM + 1
	newRLS := &TpotRecursiveLeastSquares{
		theta:  make([]float64, size),
		P:      make([][]float64, size),
		forget: r.forget,
	}
	copy(newRLS.theta, r.theta)
	for i := range r.P {
		newRLS.P[i] = make([]float64, size)
		copy(newRLS.P[i], r.P[i])
	}
	return newRLS
}
