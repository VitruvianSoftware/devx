// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import "github.com/spf13/cobra"

var mockCmd = &cobra.Command{
	Use:   "mock",
	GroupID: "telemetry",
	Short: "Manage local OpenAPI mock servers powered by Stoplight Prism",
	Long: `Spin up intelligent OpenAPI mock servers that simulate 3rd-party APIs
(Stripe, Twilio, internal services) locally based on remote OpenAPI specs.

Mock servers run as persistent background containers and are accessible
via injected environment variables (e.g. MOCK_STRIPE_URL=http://localhost:4010).`,
}

func init() {
	rootCmd.AddCommand(mockCmd)
}
