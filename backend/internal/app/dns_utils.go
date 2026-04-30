package app

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type dnsDiagnostics struct {
  A          []string `json:"a"`
  AAAA       []string `json:"aaaa"`
  CNAME      []string `json:"cname"`
  TXT        []string `json:"txt"`
  CAA        []string `json:"caa"`
  TTL        uint32   `json:"ttl"`
  CAAAllowsLE bool    `json:"caa_allows_letsencrypt"`
  MatchesServer bool  `json:"matches_server"`
  Warnings   []string `json:"warnings"`
}

func resolveDNSDiagnostics(domain, serverIP string) (dnsDiagnostics, error) {
  if strings.TrimSpace(domain) == "" {
    return dnsDiagnostics{}, errors.New("domain is required")
  }
  resolver, err := defaultDNSResolver()
  if err != nil {
    return dnsDiagnostics{}, err
  }

  aRecords, ttlA := queryDNS(resolver, dns.TypeA, domain)
  aaaaRecords, ttlAAAA := queryDNS(resolver, dns.TypeAAAA, domain)
  cnameRecords, _ := queryDNS(resolver, dns.TypeCNAME, domain)
  txtRecords, _ := queryDNS(resolver, dns.TypeTXT, domain)
  caaRecords, _ := queryDNS(resolver, dns.TypeCAA, domain)

  ttl := ttlA
  if ttl == 0 || (ttlAAAA > 0 && ttlAAAA < ttl) {
    ttl = ttlAAAA
  }

  caaAllowsLE := true
  if len(caaRecords) > 0 {
    caaAllowsLE = false
    for _, record := range caaRecords {
      if strings.Contains(strings.ToLower(record), "letsencrypt.org") {
        caaAllowsLE = true
        break
      }
    }
  }

  matchesServer := false
  serverIP = strings.TrimSpace(serverIP)
  if serverIP != "" {
    for _, ip := range append(aRecords, aaaaRecords...) {
      if ip == serverIP {
        matchesServer = true
        break
      }
    }
  }

  warnings := make([]string, 0)
  if ttl > 0 && ttl < 300 {
    warnings = append(warnings, "low_ttl")
  }
  if !caaAllowsLE {
    warnings = append(warnings, "caa_blocks_letsencrypt")
  }
  if serverIP != "" && !matchesServer {
    warnings = append(warnings, "dns_mismatch")
  }

  return dnsDiagnostics{
    A:            aRecords,
    AAAA:         aaaaRecords,
    CNAME:        cnameRecords,
    TXT:          txtRecords,
    CAA:          caaRecords,
    TTL:          ttl,
    CAAAllowsLE:  caaAllowsLE,
    MatchesServer: matchesServer,
    Warnings:     warnings,
  }, nil
}

func defaultDNSResolver() (string, error) {
  cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
  if err == nil && len(cfg.Servers) > 0 {
    return net.JoinHostPort(cfg.Servers[0], cfg.Port), nil
  }
  return "1.1.1.1:53", nil
}

func queryDNS(resolver string, qtype uint16, domain string) ([]string, uint32) {
  client := dns.Client{Timeout: 5 * time.Second}
  msg := dns.Msg{}
  msg.SetQuestion(dns.Fqdn(domain), qtype)
  response, _, err := client.Exchange(&msg, resolver)
  if err != nil || response == nil {
    return []string{}, 0
  }
  records := make([]string, 0)
  ttl := uint32(0)
  for _, answer := range response.Answer {
    if ttl == 0 || answer.Header().Ttl < ttl {
      ttl = answer.Header().Ttl
    }
    switch rr := answer.(type) {
    case *dns.A:
      records = append(records, rr.A.String())
    case *dns.AAAA:
      records = append(records, rr.AAAA.String())
    case *dns.CNAME:
      records = append(records, strings.TrimSuffix(rr.Target, "."))
    case *dns.TXT:
      records = append(records, strings.Join(rr.Txt, " "))
    case *dns.CAA:
      records = append(records, rr.Value)
    }
  }
  return records, ttl
}
