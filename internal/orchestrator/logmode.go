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

package orchestrator

import "github.com/VitruvianSoftware/devx/internal/logs"

// LogMode controls how a service node's logs are surfaced during `devx up`.
type LogMode int

const (
	LogOff      LogMode = iota // no inline output; for non-host, no file either (producer not started)
	LogRaw                     // inline raw (no prefix) + file — preserves today's host default
	LogPrefixed                // inline "[service]" prefixed+colored + file
)

// ResolveLogMode applies the opt-in precedence: flag > per-service > top-level >
// built-in default. flag/perSvc/topLevel are nil when unset. Built-in default:
// host = LogRaw (today's behavior), all other runtimes = LogOff. Any explicit
// "on" yields LogPrefixed (the multi-service case where prefixes disambiguate);
// any explicit "off" yields LogOff.
func ResolveLogMode(flag, perSvc, topLevel *bool, runtime string) LogMode {
	for _, p := range []*bool{flag, perSvc, topLevel} {
		if p != nil {
			if *p {
				return LogPrefixed
			}
			return LogOff
		}
	}
	if runtime == string(RuntimeHost) {
		return LogRaw
	}
	return LogOff
}

// sinkMode maps an orchestrator LogMode to the logs.SinkMode enum used by
// BuildSink (kept separate to avoid an import cycle: logs must not import
// orchestrator).
func sinkMode(m LogMode) logs.SinkMode {
	switch m {
	case LogRaw:
		return logs.LogRawMode
	case LogPrefixed:
		return logs.LogPrefixedMode
	default:
		return logs.LogOffMode
	}
}
