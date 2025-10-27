// Copyright The AIGW Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prediction

type TTFTPrediction interface {
	Predict(input, cached int) float64
	Train(input, cached int, y float64)
	Clone() TTFTPrediction
}

type RLS struct {
	params []float64   // Polynomial coefficients: [a, b, c, d, e, f]
	p      [][]float64 // Covariance matrix
	lambda float64     // Forgetting factor: Setting it to 1 means the forgetting mechanism is inactive, and all data contribute equally.
	n      int         // Number of parameters: Set to 6
}

// NewRLS creates a new RLS instance
// It initializes the necessary parameters for the RLS algorithm.
func NewRLS(lambda float64) *RLS {
	n := 6 // y = a*input² + b*cached² + c*input*cached + d*input + e*cached + f
	params := make([]float64, n)
	p := make([][]float64, n)
	for i := range p {
		p[i] = make([]float64, n)
		p[i][i] = 1e6
	}

	return &RLS{
		params: params,
		p:      p,
		lambda: lambda,
		n:      n,
	}
}

// Predict calculates the predicted value using the current model parameters
// and the input feature vector.
func (rls *RLS) Predict(input, cached int) float64 {
	x1 := float64(input)
	x2 := float64(cached)

	return rls.params[0]*x1*x1 +
		rls.params[1]*x2*x2 +
		rls.params[2]*x1*x2 +
		rls.params[3]*x1 +
		rls.params[4]*x2 +
		rls.params[5]*1.0
}

// Train updates the RLS model using the input data and target value.
func (rls *RLS) Train(input, cached int, y float64) {
	features := rls.constructFeatures(input, cached)
	rls.update(features, y)
}

// Construct the feature vector
func (rls *RLS) constructFeatures(input, cached int) []float64 {
	in := float64(input)
	cac := float64(cached)
	return []float64{
		in * in,
		cac * cac,
		in * cac,
		in,
		cac,
		1.0,
	}
}

// Update RLS parameters
func (rls *RLS) update(features []float64, y float64) {
	// Calculate the gain vector k
	k := make([]float64, rls.n)
	phiTP := make([]float64, rls.n)
	for i := 0; i < rls.n; i++ {
		sum := 0.0
		sum += features[0]*rls.p[0][i] +
			features[1]*rls.p[1][i] +
			features[2]*rls.p[2][i] +
			features[3]*rls.p[3][i] +
			features[4]*rls.p[4][i] +
			features[5]*rls.p[5][i]
		phiTP[i] = sum
	}

	phiTPPhi := phiTP[0]*features[0] +
		phiTP[1]*features[1] +
		phiTP[2]*features[2] +
		phiTP[3]*features[3] +
		phiTP[4]*features[4] +
		phiTP[5]*features[5]

	denominator := rls.lambda + phiTPPhi
	for i := 0; i < rls.n; i++ {
		sum := 0.0
		sum += rls.p[i][0]*features[0] +
			rls.p[i][1]*features[1] +
			rls.p[i][2]*features[2] +
			rls.p[i][3]*features[3] +
			rls.p[i][4]*features[4] +
			rls.p[i][5]*features[5]
		k[i] = sum / denominator
	}

	// Calculate the error and update the parameters
	errVal := y - features[0]*rls.params[0] -
		features[1]*rls.params[1] -
		features[2]*rls.params[2] -
		features[3]*rls.params[3] -
		features[4]*rls.params[4] -
		features[5]*rls.params[5]

	rls.params[0] += k[0] * errVal
	rls.params[1] += k[1] * errVal
	rls.params[2] += k[2] * errVal
	rls.params[3] += k[3] * errVal
	rls.params[4] += k[4] * errVal
	rls.params[5] += k[5] * errVal

	// Update the covariance matrix P
	for i := 0; i < rls.n; i++ {
		rls.p[i][0] = (rls.p[i][0] - k[i]*phiTP[0]) / rls.lambda
		rls.p[i][1] = (rls.p[i][1] - k[i]*phiTP[1]) / rls.lambda
		rls.p[i][2] = (rls.p[i][2] - k[i]*phiTP[2]) / rls.lambda
		rls.p[i][3] = (rls.p[i][3] - k[i]*phiTP[3]) / rls.lambda
		rls.p[i][4] = (rls.p[i][4] - k[i]*phiTP[4]) / rls.lambda
		rls.p[i][5] = (rls.p[i][5] - k[i]*phiTP[5]) / rls.lambda
	}
}

func (rls *RLS) Clone() TTFTPrediction {
	newRLS := &RLS{
		params: make([]float64, rls.n),
		p:      make([][]float64, rls.n),
		lambda: rls.lambda,
		n:      rls.n,
	}
	copy(newRLS.params, rls.params)
	for i := range rls.p {
		newRLS.p[i] = make([]float64, rls.n)
		copy(newRLS.p[i], rls.p[i])
	}
	return newRLS
}
