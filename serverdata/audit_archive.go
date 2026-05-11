package serverdata

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

// AuditArchive implements Source by reading from an audit archive
// Filtering: every endpoint applies opts.EventFilterOptions to the captured
// server.ServerInfo of each artifact:
//   - Name matches against either Server.Name or Server.ID
//   - Host and Cluster match against the corresponding ServerInfo fields
//   - ExactMatch toggles equality vs. substring matching for Name/Host/Cluster
//   - Domain is always compared exactly
//   - Tags are case-insensitive and must all be present (AND)
//
// Endpoints whose live counterpart accepts options that the archive cannot
// satisfy from captured data (e.g. ConnzOptions.FilterSubject, GatewayzOptions.Name,
// LeafzOptions.Account, JszOptions.Account, AccountzOptions.Account when no per-account
// snapshot was gathered, HealthzOptions.JSEnabledOnly/JSServerOnly/Account/Stream/Consumer)
// return ErrUnsupportedOption identifying the unsupported fields rather than
// silently producing partial or incorrect results.
type AuditArchive struct {
	reader      *archive.Reader
	archivePath string
}

// NewAuditArchive opens an audit archive file and returns a Source backed by it.
// archivePath is required; passing an empty string returns an error.
func NewAuditArchive(archivePath string) (*AuditArchive, error) {
	if archivePath == "" {
		return nil, fmt.Errorf("archivePath is required")
	}
	r, err := archive.NewReader(archivePath)
	if err != nil {
		return nil, err
	}
	return &AuditArchive{reader: r, archivePath: archivePath}, nil
}

// Varz builds and filters a Varz response from the archive
func (a *AuditArchive) Varz(opts server.VarzEventOptions) ([]*server.ServerAPIVarzResponse, error) {
	var results []*server.ServerAPIVarzResponse
	_, err := a.reader.EachClusterServerVarz(func(_ *archive.Tag, _ *archive.Tag, cbError error, vz *server.ServerAPIVarzResponse) error {
		if cbError != nil || !matchesFilter(vz.Server, &opts.EventFilterOptions) {
			return nil
		}
		results = append(results, vz)
		return nil
	})
	return results, err
}

// unsupported returns an ErrUnsupportedOption error listing the fields that
// the archive cannot honor for the given method.
func unsupported(method string, fields ...string) error {
	return fmt.Errorf("%w for archive %s: %s", ErrUnsupportedOption, method, strings.Join(fields, ", "))
}

// Connz builds and filters a Connz response from the archive
func (a *AuditArchive) Connz(opts server.ConnzEventOptions) ([]*server.ServerAPIConnzResponse, error) {
	var fields []string
	if opts.Account != "" {
		fields = append(fields, "account")
	}
	if opts.FilterSubject != "" {
		fields = append(fields, "filter_subject")
	}
	if opts.User != "" {
		fields = append(fields, "user")
	}
	if opts.State != 0 {
		fields = append(fields, "state")
	}
	if len(fields) > 0 {
		return nil, unsupported("connz", fields...)
	}

	seen := map[string]*server.ServerAPIConnzResponse{}
	var results []*server.ServerAPIConnzResponse

	for _, accountName := range a.reader.AccountNames() {
		tags := []*archive.Tag{
			archive.TagAccount(accountName),
			archive.TagAccountConnections(),
		}
		err := archive.ForEachTaggedArtifact(a.reader, tags, func(cz *server.ServerAPIConnzResponse) error {
			if !matchesFilter(cz.Server, &opts.EventFilterOptions) {
				return nil
			}
			if cz.Server == nil || cz.Data == nil {
				return nil
			}
			for _, conn := range cz.Data.Conns {
				if conn.Account == "" {
					conn.Account = accountName
				}
			}
			key := cz.Server.ID
			if base, ok := seen[key]; ok {
				base.Data.Conns = append(base.Data.Conns, cz.Data.Conns...)
				base.Data.NumConns = len(base.Data.Conns)
				base.Data.Total += cz.Data.Total
			} else {
				seen[key] = cz
				results = append(results, cz)
			}
			return nil
		})
		if err != nil && err != archive.ErrNoMatches {
			return nil, err
		}
	}

	return results, nil
}

// Routez builds and filters a Routez response from the archive
func (a *AuditArchive) Routez(opts server.RoutezEventOptions) ([]*server.ServerAPIRoutezResponse, error) {
	var results []*server.ServerAPIRoutezResponse
	_, err := archive.EachClusterServerArtifact(a.reader, archive.TagServerRoutes(),
		func(_ *archive.Tag, _ *archive.Tag, cbError error, rz *server.ServerAPIRoutezResponse) error {
			if cbError != nil || !matchesFilter(rz.Server, &opts.EventFilterOptions) {
				return nil
			}
			results = append(results, rz)
			return nil
		})
	return results, err
}

// Gatewayz builds and filters a Gatewayz response from the archive
func (a *AuditArchive) Gatewayz(opts server.GatewayzEventOptions) ([]*server.ServerAPIGatewayzResponse, error) {
	if opts.GatewayzOptions.Name != "" {
		return nil, unsupported("gatewayz", "name")
	}

	var results []*server.ServerAPIGatewayzResponse
	_, err := archive.EachClusterServerArtifact(a.reader, archive.TagServerGateways(),
		func(_ *archive.Tag, _ *archive.Tag, cbError error, gz *server.ServerAPIGatewayzResponse) error {
			if cbError != nil || !matchesFilter(gz.Server, &opts.EventFilterOptions) {
				return nil
			}
			results = append(results, gz)
			return nil
		})
	return results, err
}

// Leafz builds and filters a Leafz response from the archive
func (a *AuditArchive) Leafz(opts server.LeafzEventOptions) ([]*server.ServerAPILeafzResponse, error) {
	if opts.Account != "" {
		return nil, unsupported("leafz", "account")
	}

	var results []*server.ServerAPILeafzResponse
	_, err := a.reader.EachClusterServerLeafz(func(_ *archive.Tag, _ *archive.Tag, cbError error, lz *server.ServerAPILeafzResponse) error {
		if cbError != nil || !matchesFilter(lz.Server, &opts.EventFilterOptions) {
			return nil
		}
		results = append(results, lz)
		return nil
	})
	return results, err
}

// Subsz builds and filters a Subsz response from the archive
func (a *AuditArchive) Subsz(opts server.SubszEventOptions) ([]*server.ServerAPISubszResponse, error) {
	seen := map[string]*server.ServerAPISubszResponse{}
	var results []*server.ServerAPISubszResponse

	_, err := archive.EachClusterServerArtifact(a.reader, archive.TagServerSubs(),
		func(clusterTag *archive.Tag, serverTag *archive.Tag, cbError error, sz *server.ServerAPISubszResponse) error {
			if cbError != nil || !matchesFilter(sz.Server, &opts.EventFilterOptions) {
				return nil
			}
			if sz.Data == nil {
				return nil
			}
			key := serverKey(clusterTag, serverTag)
			if base, ok := seen[key]; ok {
				base.Data.Subs = append(base.Data.Subs, sz.Data.Subs...)
			} else {
				seen[key] = sz
				results = append(results, sz)
			}
			return nil
		})
	return results, err
}

// Jsz builds and filters a Jsz response from the archive
func (a *AuditArchive) Jsz(opts server.JszEventOptions) ([]*server.ServerAPIJszResponse, error) {
	if opts.Account != "" {
		return nil, unsupported("jsz", "account")
	}

	seen := map[string]*server.ServerAPIJszResponse{}
	var results []*server.ServerAPIJszResponse

	_, err := a.reader.EachClusterServerJsz(func(clusterTag *archive.Tag, serverTag *archive.Tag, cbError error, jsz *server.ServerAPIJszResponse) error {
		if cbError != nil || !matchesFilter(jsz.Server, &opts.EventFilterOptions) {
			return nil
		}
		if jsz.Data == nil {
			return nil
		}
		key := serverKey(clusterTag, serverTag)
		if base, ok := seen[key]; ok {
			base.Data.AccountDetails = append(base.Data.AccountDetails, jsz.Data.AccountDetails...)
		} else {
			seen[key] = jsz
			results = append(results, jsz)
		}
		return nil
	})
	return results, err
}

// Healthz builds and filters a Healthz response from the archive
func (a *AuditArchive) Healthz(opts server.HealthzEventOptions) ([]*server.ServerAPIHealthzResponse, error) {
	var fields []string
	if opts.JSEnabledOnly {
		fields = append(fields, "js-enabled-only")
	}
	if opts.JSServerOnly {
		fields = append(fields, "js-server-only")
	}
	if opts.Account != "" {
		fields = append(fields, "account")
	}
	if opts.Stream != "" {
		fields = append(fields, "stream")
	}
	if opts.Consumer != "" {
		fields = append(fields, "consumer")
	}
	if len(fields) > 0 {
		return nil, unsupported("healthz", fields...)
	}

	var results []*server.ServerAPIHealthzResponse
	_, err := a.reader.EachClusterServerHealthz(func(_ *archive.Tag, _ *archive.Tag, cbError error, hz *server.ServerAPIHealthzResponse) error {
		if cbError != nil || !matchesFilter(hz.Server, &opts.EventFilterOptions) {
			return nil
		}
		results = append(results, hz)
		return nil
	})
	return results, err
}

type accountInfoResponse struct {
	Server *server.ServerInfo  `json:"server"`
	Data   *server.AccountInfo `json:"data,omitempty"`
}

// Accountz builds and filters an Accountz response from the archive.
func (a *AuditArchive) Accountz(opts server.AccountzEventOptions) ([]*server.ServerAPIAccountzResponse, error) {
	if opts.AccountzOptions.Account != "" {
		return a.accountzDetail(opts)
	}

	var results []*server.ServerAPIAccountzResponse
	_, err := a.reader.EachClusterServerAccountz(func(_ *archive.Tag, _ *archive.Tag, cbError error, az *server.ServerAPIAccountzResponse) error {
		if cbError != nil || !matchesFilter(az.Server, &opts.EventFilterOptions) {
			return nil
		}
		results = append(results, az)
		return nil
	})
	return results, err
}

// accountzDetail reads per-account account_info artifacts and wraps them.
//
// An empty result mirrors live behavior for an unknown account but cannot be
// distinguished from cases where gather failed to capture INFO for that
// account (network error, permission issue, account only existed on servers
// that were not gathered).
func (a *AuditArchive) accountzDetail(opts server.AccountzEventOptions) ([]*server.ServerAPIAccountzResponse, error) {
	var results []*server.ServerAPIAccountzResponse
	tags := []*archive.Tag{
		archive.TagAccount(opts.AccountzOptions.Account),
		archive.TagAccountInfo(),
	}

	err := archive.ForEachTaggedArtifact(a.reader, tags, func(air *accountInfoResponse) error {
		if !matchesFilter(air.Server, &opts.EventFilterOptions) {
			return nil
		}
		resp := &server.ServerAPIAccountzResponse{
			Server: air.Server,
			Data: &server.Accountz{
				Account: air.Data,
			},
		}
		results = append(results, resp)
		return nil
	})
	if err == archive.ErrNoMatches {
		return results, nil
	}
	return results, err
}

// Statz builds and filters a Statz response from the archive
// TODO(ploubser:) ActiveAccounts, StaleConnections, StaleConnectionStats, StalledClients
// and ActiveServers are not available in archive data and cannot be added to the response.
func (a *AuditArchive) Statz(opts server.StatszEventOptions) ([]*server.ServerStatsMsg, error) {
	varzResponses, err := a.Varz(server.VarzEventOptions{EventFilterOptions: opts.EventFilterOptions})
	if err != nil {
		return nil, err
	}

	routesByID := map[string]*server.Routez{}
	routezResponses, err := a.Routez(server.RoutezEventOptions{EventFilterOptions: opts.EventFilterOptions})
	if err != nil && err != archive.ErrNoMatches {
		return nil, err
	}
	for _, r := range routezResponses {
		if r.Server != nil {
			routesByID[r.Server.ID] = r.Data
		}
	}

	gatewaysByID := map[string]*server.Gatewayz{}
	gatewayzResponses, err := a.Gatewayz(server.GatewayzEventOptions{EventFilterOptions: opts.EventFilterOptions})
	if err != nil && err != archive.ErrNoMatches {
		return nil, err
	}
	for _, g := range gatewayzResponses {
		if g.Server != nil {
			gatewaysByID[g.Server.ID] = g.Data
		}
	}

	var results []*server.ServerStatsMsg
	for _, vr := range varzResponses {
		if vr.Server == nil || vr.Data == nil {
			continue
		}

		msg := &server.ServerStatsMsg{
			Server: *vr.Server,
			Stats: server.ServerStats{
				Start:            vr.Data.Start,
				Mem:              vr.Data.Mem,
				Cores:            vr.Data.Cores,
				CPU:              vr.Data.CPU,
				Connections:      vr.Data.Connections,
				TotalConnections: vr.Data.TotalConnections,
				NumSubs:          vr.Data.Subscriptions,
				Sent: server.DataStats{
					MsgBytes: server.MsgBytes{Msgs: vr.Data.OutMsgs, Bytes: vr.Data.OutBytes},
				},
				Received: server.DataStats{
					MsgBytes: server.MsgBytes{Msgs: vr.Data.InMsgs, Bytes: vr.Data.InBytes},
				},
				SlowConsumers:      vr.Data.SlowConsumers,
				SlowConsumersStats: vr.Data.SlowConsumersStats,
				MemLimit:           vr.Data.MemLimit,
				MaxProcs:           vr.Data.MaxProcs,
			},
		}

		if vr.Data.JetStream.Config != nil {
			msg.Stats.JetStream = &vr.Data.JetStream
		}

		if rz, ok := routesByID[vr.Server.ID]; ok {
			for _, ri := range rz.Routes {
				msg.Stats.Routes = append(msg.Stats.Routes, &server.RouteStat{
					ID:   ri.Rid,
					Name: ri.RemoteName,
					Sent: server.DataStats{
						MsgBytes: server.MsgBytes{Msgs: ri.OutMsgs, Bytes: ri.OutBytes},
					},
					Received: server.DataStats{
						MsgBytes: server.MsgBytes{Msgs: ri.InMsgs, Bytes: ri.InBytes},
					},
					Pending: ri.Pending,
				})
			}
		}

		if gz, ok := gatewaysByID[vr.Server.ID]; ok {
			gwStats := map[string]*server.GatewayStat{}

			for gname, conns := range gz.InboundGateways {
				stat := gwStats[gname]
				if stat == nil {
					stat = &server.GatewayStat{Name: gname}
					gwStats[gname] = stat
				}
				stat.NumInbound += len(conns)
				for _, c := range conns {
					if c.Connection != nil {
						stat.Received.Msgs += c.Connection.InMsgs
						stat.Received.Bytes += c.Connection.InBytes
					}
				}
			}

			for gname, gw := range gz.OutboundGateways {
				stat := gwStats[gname]
				if stat == nil {
					stat = &server.GatewayStat{Name: gname}
					gwStats[gname] = stat
				}
				if gw.Connection != nil {
					stat.Sent.Msgs += gw.Connection.OutMsgs
					stat.Sent.Bytes += gw.Connection.OutBytes
				}
			}

			for _, stat := range gwStats {
				msg.Stats.Gateways = append(msg.Stats.Gateways, stat)
			}
		}

		results = append(results, msg)
	}

	return results, nil
}

// CollectAccounts gathers account-level JetStream metadata from the archive
func (a *AuditArchive) CollectAccounts() ([]*server.AccountDetail, error) {
	jszResponses, err := a.Jsz(server.JszEventOptions{
		JSzOptions: server.JSzOptions{
			Accounts: true,
			Streams:  true,
			Consumer: true,
			Config:   true,
		},
	})
	if err != nil {
		return nil, err
	}

	accountMap := map[string]*server.AccountDetail{}
	for _, jsz := range jszResponses {
		if jsz.Data == nil {
			continue
		}
		for _, acct := range jsz.Data.AccountDetails {
			if existing, found := accountMap[acct.Name]; found {
				existing.Streams = mergeStreams(existing.Streams, acct.Streams)
			} else {
				cp := *acct
				accountMap[acct.Name] = &cp
			}
		}
	}

	accounts := make([]*server.AccountDetail, 0, len(accountMap))
	for _, acct := range accountMap {
		accounts = append(accounts, acct)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	for _, acct := range accounts {
		sort.Slice(acct.Streams, func(i, j int) bool { return acct.Streams[i].Name < acct.Streams[j].Name })
	}

	return accounts, nil
}

// Ipqueuesz reads ipqueuesz data from the archive. The archive can only narrow
// what gather captured: if gather called with All=false or a non-empty Filter,
// the captured data is already pre-narrowed and broader queries cannot recover it.
func (a *AuditArchive) Ipqueuesz(opts server.IpqueueszEventOptions) ([]*server.ServerAPIpqueueszResponse, error) {
	var results []*server.ServerAPIpqueueszResponse
	tags := []*archive.Tag{archive.TagAccountIpqueuesz()}
	err := archive.ForEachTaggedArtifact(a.reader, tags, func(r *server.ServerAPIpqueueszResponse) error {
		if !matchesFilter(r.Server, &opts.EventFilterOptions) {
			return nil
		}
		if r.Data != nil {
			for name, ipq := range *r.Data {
				if !opts.All && ipq.Pending == 0 && ipq.InProgress == 0 {
					delete(*r.Data, name)
					continue
				}
				if opts.Filter != "" && !strings.Contains(name, opts.Filter) {
					delete(*r.Data, name)
				}
			}
		}
		results = append(results, r)
		return nil
	})
	if err == archive.ErrNoMatches {
		return results, nil
	}
	return results, err
}

// Raftz reads raftz data from the archive. The archive can only narrow what
// gather captured. Live mode defaults an empty AccountFilter to the system
// account; the archive cannot replicate that default because it doesn't know
// which account was the system account at gather time, so an empty
// AccountFilter here means "all captured accounts".
func (a *AuditArchive) Raftz(opts server.RaftzEventOptions) ([]*server.ServerAPIRaftzResponse, error) {
	var results []*server.ServerAPIRaftzResponse
	tags := []*archive.Tag{archive.TagAccountRaftz()}
	err := archive.ForEachTaggedArtifact(a.reader, tags, func(r *server.ServerAPIRaftzResponse) error {
		if !matchesFilter(r.Server, &opts.EventFilterOptions) {
			return nil
		}
		if r.Data != nil {
			for accountName, groups := range *r.Data {
				if opts.AccountFilter != "" && accountName != opts.AccountFilter {
					delete(*r.Data, accountName)
					continue
				}
				if opts.GroupFilter != "" {
					for groupID := range groups {
						if groupID != opts.GroupFilter {
							delete(groups, groupID)
						}
					}
					if len(groups) == 0 {
						delete(*r.Data, accountName)
					}
				}
			}
		}
		results = append(results, r)
		return nil
	})
	if err == archive.ErrNoMatches {
		return results, nil
	}
	return results, err
}

// Profilez returns profile artifacts captured by audit gather. opts.Duration,
// Host, Domain, and Tags are rejected with ErrUnsupportedOption. Errored
// captures are absent because gather drops them. opts.Debug must match the
// level audit gather stored exactly.
func (a *AuditArchive) Profilez(opts server.ProfilezEventOptions) ([]*ProfilezResponse, error) {
	var fields []string
	if opts.Duration != 0 {
		fields = append(fields, "duration")
	}
	if opts.Host != "" {
		fields = append(fields, "host")
	}
	if opts.Domain != "" {
		fields = append(fields, "domain")
	}
	if len(opts.Tags) > 0 {
		fields = append(fields, "tags")
	}
	if len(fields) > 0 {
		return nil, unsupported("profilez", fields...)
	}

	// Recover full ServerInfo from Varz when present. Archive tag values
	// cannot contain "/" (enforced by audit/archive validatePathComponent),
	// so it is safe as a key separator.
	fullInfo := map[string]*server.ServerInfo{}
	if vzs, vErr := a.Varz(server.VarzEventOptions{}); vErr == nil {
		for _, vz := range vzs {
			if vz.Server != nil {
				fullInfo[vz.Server.Cluster+"/"+vz.Server.Name] = vz.Server
			}
		}
	}

	var results []*ProfilezResponse
	err := forEachProfileArtifact(a.archivePath, func(cluster, srv, rawName string, readBytes func() ([]byte, error)) error {
		name, debug := splitProfileName(rawName)
		if opts.ProfilezOptions.Name != "" && opts.ProfilezOptions.Name != name {
			return nil
		}
		if opts.Debug != debug {
			return nil
		}

		info := fullInfo[cluster+"/"+srv]
		if info == nil {
			info = &server.ServerInfo{Name: srv, Cluster: cluster}
		}
		if !matchesFilter(info, &opts.EventFilterOptions) {
			return nil
		}

		raw, err := readBytes()
		if err != nil {
			return err
		}

		results = append(results, &ProfilezResponse{
			Server: info,
			Data:   &server.ProfilezStatus{Profile: raw},
		})
		return nil
	})
	return results, err
}

func forEachProfileArtifact(archivePath string, cb func(cluster, server, name string, readBytes func() ([]byte, error)) error) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	var manifestFile *zip.File
	for _, f := range zr.File {
		if f.Name == "capture/misc/manifest.json" {
			manifestFile = f
			break
		}
	}
	if manifestFile == nil {
		return fmt.Errorf("archive %s: manifest not found at expected path", archivePath)
	}

	mrc, err := manifestFile.Open()
	if err != nil {
		return fmt.Errorf("open manifest in %s: %w", archivePath, err)
	}
	var manifest map[string][]archive.Tag
	if err := json.NewDecoder(mrc).Decode(&manifest); err != nil {
		mrc.Close()
		return fmt.Errorf("decode manifest in %s: %w", archivePath, err)
	}
	mrc.Close()

	fileMap := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		fileMap[f.Name] = f
	}

	type entry struct{ cluster, server, name, fname string }
	var entries []entry
	for fname, tags := range manifest {
		var cluster, srv, profileName string
		isProfile := false
		for _, t := range tags {
			switch string(t.Name) {
			case "artifact_type":
				if t.Value == "profile" {
					isProfile = true
				}
			case "cluster":
				cluster = t.Value
			case "server":
				srv = t.Value
			case "profile_name":
				profileName = t.Value
			}
		}
		if !isProfile {
			continue
		}
		if cluster == "unclustered" {
			cluster = ""
		}
		if _, ok := fileMap[fname]; !ok {
			continue
		}
		entries = append(entries, entry{cluster, srv, profileName, fname})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].cluster != entries[j].cluster {
			return entries[i].cluster < entries[j].cluster
		}
		if entries[i].server != entries[j].server {
			return entries[i].server < entries[j].server
		}
		return entries[i].name < entries[j].name
	})

	for _, e := range entries {
		readBytes := func() ([]byte, error) {
			rc, err := fileMap[e.fname].Open()
			if err != nil {
				return nil, fmt.Errorf("open %s: %w", e.fname, err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
		if err := cb(e.cluster, e.server, e.name, readBytes); err != nil {
			return err
		}
	}
	return nil
}

// splitProfileName reverses gather's "<name>[_<debug>]" encoding.
func splitProfileName(tag string) (name string, debug int) {
	if i := strings.LastIndex(tag, "_"); i > 0 {
		if n, err := strconv.Atoi(tag[i+1:]); err == nil && n >= 0 {
			return tag[:i], n
		}
	}
	return tag, 0
}

func (a *AuditArchive) Close() error {
	return a.reader.Close()
}

func serverKey(clusterTag, serverTag *archive.Tag) string {
	return clusterTag.Value + "/" + serverTag.Value
}

// matchesFilter checks whether a server matches the given EventFilterOptions.
func matchesFilter(info *server.ServerInfo, f *server.EventFilterOptions) bool {
	if info == nil || f == nil {
		return true
	}

	if f.Name != "" && !matchField(info.Name, f.Name, f.ExactMatch) && !matchField(info.ID, f.Name, f.ExactMatch) {
		return false
	}
	if f.Host != "" && !matchField(info.Host, f.Host, f.ExactMatch) {
		return false
	}
	if f.Cluster != "" && !matchField(info.Cluster, f.Cluster, f.ExactMatch) {
		return false
	}
	if f.Domain != "" && info.Domain != f.Domain {
		return false
	}
	for _, ft := range f.Tags {
		if !hasTag(info.Tags, ft) {
			return false
		}
	}

	return true
}

func matchField(value, filter string, exact bool) bool {
	if exact {
		return value == filter
	}
	return strings.Contains(value, filter)
}

func hasTag(tags []string, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	for _, t := range tags {
		if strings.ToLower(t) == filter {
			return true
		}
	}
	return false
}
