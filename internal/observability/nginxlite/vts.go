package nginxlite

import (
	"encoding/json"
	"fmt"
)

type vtsPayload struct {
	ServerZones   map[string]vtsZone           `json:"serverZones"`
	UpstreamZones map[string][]vtsUpstreamPeer `json:"upstreamZones"`
}

type vtsZone struct {
	RequestCounter json.Number `json:"requestCounter"`
	InBytes        json.Number `json:"inBytes"`
	OutBytes       json.Number `json:"outBytes"`
	RequestMsec    json.Number `json:"requestMsec"`
}

type vtsUpstreamPeer struct {
	Server         string      `json:"server"`
	RequestCounter json.Number `json:"requestCounter"`
	InBytes        json.Number `json:"inBytes"`
	OutBytes       json.Number `json:"outBytes"`
	ResponseMsec   json.Number `json:"responseMsec"`
	Down           bool        `json:"down"`
}

// VTSSnapshot is a parsed VTS JSON body.
type VTSSnapshot struct {
	Servers    []VTSServerRow
	Upstreams  []VTSUpstreamRow
}

// VTSServerRow is a normalized server zone row.
type VTSServerRow struct {
	ServerName  string
	Requests    int
	InBytes     int
	OutBytes    int
	RequestMsec float64
}

// VTSUpstreamRow is a normalized upstream peer row.
type VTSUpstreamRow struct {
	UpstreamName string
	ServerAddr   string
	Requests     int
	InBytes      int
	OutBytes     int
	ResponseMsec float64
	Down         bool
}

// ParseVTSJSON parses nginx-module-vts JSON output.
func ParseVTSJSON(body []byte) (VTSSnapshot, error) {
	var payload vtsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return VTSSnapshot{}, fmt.Errorf("parse vts json: %w", err)
	}
	out := VTSSnapshot{}
	for name, zone := range payload.ServerZones {
		if name == "*" {
			continue
		}
		out.Servers = append(out.Servers, VTSServerRow{
			ServerName:  name,
			Requests:    numberInt(zone.RequestCounter),
			InBytes:     numberInt(zone.InBytes),
			OutBytes:    numberInt(zone.OutBytes),
			RequestMsec: numberFloat(zone.RequestMsec),
		})
	}
	for upstreamName, peers := range payload.UpstreamZones {
		for _, peer := range peers {
			out.Upstreams = append(out.Upstreams, VTSUpstreamRow{
				UpstreamName: upstreamName,
				ServerAddr:   peer.Server,
				Requests:     numberInt(peer.RequestCounter),
				InBytes:      numberInt(peer.InBytes),
				OutBytes:     numberInt(peer.OutBytes),
				ResponseMsec: numberFloat(peer.ResponseMsec),
				Down:         peer.Down,
			})
		}
	}
	if len(out.Servers) == 0 && len(out.Upstreams) == 0 && len(body) > 0 && !json.Valid(body) {
		return VTSSnapshot{}, fmt.Errorf("unrecognized vts json")
	}
	return out, nil
}

func numberInt(n json.Number) int {
	if n == "" {
		return 0
	}
	v, err := n.Int64()
	if err != nil {
		f, _ := n.Float64()
		return int(f)
	}
	return int(v)
}

func numberFloat(n json.Number) float64 {
	if n == "" {
		return 0
	}
	f, err := n.Float64()
	if err != nil {
		return 0
	}
	return f
}
