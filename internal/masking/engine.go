package masking

import (
	"net"
	"regexp"
	"strings"
)

var (
	uuidRE   = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	ipv4RE   = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	ipv6RE   = regexp.MustCompile(`[0-9a-fA-F]{0,4}(?::[0-9a-fA-F]{0,4}){2,7}`)
	hashRE   = regexp.MustCompile(`\b[0-9a-fA-F]{128}\b|\b[0-9a-fA-F]{96}\b|\b[0-9a-fA-F]{64}\b|\b[0-9a-fA-F]{40}\b`)
	tpmKeyRE = regexp.MustCompile(`"(aik_tpm|ek_tpm|ekcert|mtls_cert)"\s*:\s*"[^"]*"`)
	aliasRE  = regexp.MustCompile(`\b(AGENT|HOST|HASH|TPM)-\d+\b`)
)

type Engine struct {
	enabled bool
	agents  *AliasMap
	hosts   *AliasMap
	hashes  *AliasMap
	tpmKeys *AliasMap
}

func NewEngine(enabled bool) *Engine {
	return &Engine{
		enabled: enabled,
		agents:  NewAliasMap("AGENT"),
		hosts:   NewAliasMap("HOST"),
		hashes:  NewAliasMap("HASH"),
		tpmKeys: NewAliasMap("TPM"),
	}
}

func (e *Engine) Enabled() bool {
	return e.enabled
}

func (e *Engine) Mask(text string) string {
	if !e.enabled {
		return text
	}

	text = tpmKeyRE.ReplaceAllStringFunc(text, func(match string) string {
		if before, value, ok := strings.Cut(match, `":"`); ok {
			value = strings.TrimSuffix(value, `"`)
			return before + `":"` + e.tpmKeys.GetOrCreate(value) + `"`
		}
		if before, value, ok := strings.Cut(match, `": "`); ok {
			value = strings.TrimSuffix(value, `"`)
			return before + `": "` + e.tpmKeys.GetOrCreate(value) + `"`
		}
		return match
	})

	text = uuidRE.ReplaceAllStringFunc(text, func(match string) string {
		return e.agents.GetOrCreate(match)
	})

	maskIP := func(match string) string {
		if net.ParseIP(match) == nil {
			return match
		}
		return e.hosts.GetOrCreate(match)
	}
	text = ipv4RE.ReplaceAllStringFunc(text, maskIP)
	text = ipv6RE.ReplaceAllStringFunc(text, maskIP)

	text = hashRE.ReplaceAllStringFunc(text, func(match string) string {
		return e.hashes.GetOrCreate(match)
	})

	return text
}

func (e *Engine) Unmask(text string) string {
	if !e.enabled {
		return text
	}

	maps := []*AliasMap{e.agents, e.hosts, e.hashes, e.tpmKeys}
	return aliasRE.ReplaceAllStringFunc(text, func(match string) string {
		for _, m := range maps {
			if real, ok := m.Resolve(match); ok {
				return real
			}
		}
		return match
	})
}
