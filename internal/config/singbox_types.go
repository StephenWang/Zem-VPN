package config

// ProxyTypes 是 sing-box 中所有被识别为实际代理节点的 outbound 类型
var ProxyTypes = []string{
	"shadowsocks",
	"shadowsocksr",
	"vmess",
	"vless",
	"trojan",
	"socks",
	"http",
	"hysteria",
	"hysteria2",
	"tuic",
	"warp",
	"wireguard",
	"anytls",
	"shadowtls",
	"ssh",
	"trojan-go",
}

type SingBoxConfig struct {
	Log          *LogOptions          `json:"log,omitempty"`
	DNS          *DNSOptions          `json:"dns,omitempty"`
	Inbounds     []Inbound            `json:"inbounds"`
	Outbounds    []Outbound           `json:"outbounds"`
	Route        RouteOptions         `json:"route"`
	Experimental *ExperimentalOptions `json:"experimental,omitempty"`
}

type ExperimentalOptions struct {
	V2RayAPI *V2RayAPIOptions `json:"v2ray_api,omitempty"`
}

type V2RayAPIOptions struct {
	Listen string             `json:"listen,omitempty"`
	Stats  *V2RayStatsOptions `json:"stats,omitempty"`
}

type V2RayStatsOptions struct {
	Enabled  bool     `json:"enabled,omitempty"`
	Inbounds []string `json:"inbounds,omitempty"`
}

type LogOptions struct {
	Level  string `json:"level"`
	Output string `json:"output,omitempty"`
}

type DNSOptions struct {
	Servers []DNSServer `json:"servers"`
	Rules   []DNSRule   `json:"rules"`
}

type DNSServer struct {
	Type           string                 `json:"type"`
	Tag            string                 `json:"tag"`
	Server         string                 `json:"server,omitempty"`
	ServerPort     int                    `json:"server_port,omitempty"`
	Detour         string                 `json:"detour,omitempty"`
	DomainResolver *DomainResolverOptions `json:"domain_resolver,omitempty"`
}

type DomainResolverOptions struct {
	Server string `json:"server"`
}

type DNSRule struct {
	Action  string   `json:"action,omitempty"`
	Server  string   `json:"server,omitempty"`
	RuleSet []string `json:"rule_set,omitempty"`
	Rule    `json:",inline"`
}

type Inbound struct {
	Type                   string   `json:"type"`
	Tag                    string   `json:"tag"`
	Address                []string `json:"address,omitempty"`
	Stack                  string   `json:"stack,omitempty"`
	AutoRoute              bool     `json:"auto_route,omitempty"`
	StrictRoute            bool     `json:"strict_route,omitempty"`
	EndpointIndependentNAT bool     `json:"endpoint_independent_nat,omitempty"`
	Sniff                  bool     `json:"sniff,omitempty"`
	Listen                 string   `json:"listen,omitempty"`
	ListenPort             int      `json:"listen_port,omitempty"`
	MTU                    int      `json:"mtu,omitempty"`
	GSO                    bool     `json:"gso,omitempty"`
}

type Outbound struct {
	Type            string                 `json:"type"`
	Tag             string                 `json:"tag"`
	Server          string                 `json:"server,omitempty"`
	ServerPort      int                    `json:"server_port,omitempty"`
	UUID            string                 `json:"uuid,omitempty"`
	AlterID         int                    `json:"alterId,omitempty"`
	Security        string                 `json:"security,omitempty"`
	Method          string                 `json:"method,omitempty"`
	Password        string                 `json:"password,omitempty"`
	Username        string                 `json:"username,omitempty"`
	Plugin          string                 `json:"plugin,omitempty"`
	PluginOpts      map[string]interface{} `json:"plugin_opts,omitempty"`
	Transport       *Transport             `json:"transport,omitempty"`
	TLS             *TLSOptions            `json:"tls,omitempty"`
	Mux             *MuxOptions            `json:"multiplex,omitempty"`
	Outbounds       []string               `json:"outbounds,omitempty"`
	Default         string                 `json:"default,omitempty"`
	Version         string                 `json:"version,omitempty"`
	Reserved        []int                  `json:"reserved,omitempty"`
	LocalAddress    []string               `json:"local_address,omitempty"`
	PrivateKey      string                 `json:"private_key,omitempty"`
	PublicKey       string                 `json:"public_key,omitempty"`
	PeerPublicKey   string                 `json:"peer_public_key,omitempty"`
	PreSharedKey    string                 `json:"pre_shared_key,omitempty"`
	MTU             int                    `json:"mtu,omitempty"`
	IdleTimeout     string                 `json:"idle_timeout,omitempty"`
	Congestion      string                 `json:"congestion,omitempty"`
	UpMbps          int                    `json:"up_mbps,omitempty"`
	DownMbps        int                    `json:"down_mbps,omitempty"`
	Obfs            *Hysteria2ObfsOptions  `json:"obfs,omitempty"`
	ObfsPassword    string                 `json:"obfs_password,omitempty"`
}

type Transport struct {
	Type        string            `json:"type"`
	Path        string            `json:"path,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type TLSOptions struct {
	Enabled     bool            `json:"enabled"`
	ServerName  string          `json:"server_name,omitempty"`
	Insecure    bool            `json:"insecure,omitempty"`
	ALPN        []string        `json:"alpn,omitempty"`
	UTLS        *UTLSOptions    `json:"utls,omitempty"`
	Reality     *RealityOptions `json:"reality,omitempty"`
}

type UTLSOptions struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type Hysteria2ObfsOptions struct {
	Type     string `json:"type"`
	Password string `json:"password,omitempty"`
}

type RealityOptions struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty"`
}

type MuxOptions struct {
	Enabled        bool   `json:"enabled"`
	Protocol       string `json:"protocol,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
	MinStreams     int    `json:"min_streams,omitempty"`
	MaxStreams     int    `json:"max_streams,omitempty"`
	Padding        bool   `json:"padding,omitempty"`
}

type RouteOptions struct {
	AutoDetectInterface bool        `json:"auto_detect_interface"`
	Final               string      `json:"final,omitempty"`
	Rules               []RouteRule `json:"rules"`
}

type RouteRule struct {
	Action   string `json:"action,omitempty"`
	Outbound string `json:"outbound,omitempty"`
	Rule     `json:",inline"`
}

type Rule struct {
	Domain         []string `json:"domain,omitempty"`
	DomainSuffix   []string `json:"domain_suffix,omitempty"`
	DomainKeyword  []string `json:"domain_keyword,omitempty"`
	IPCIDR         []string `json:"ip_cidr,omitempty"`
	SourceIPCIDR   []string `json:"source_ip_cidr,omitempty"`
	GeoIP          []string `json:"geoip,omitempty"`
	GeoSite        []string `json:"geosite,omitempty"`
	Port           []int    `json:"port,omitempty"`
	SourcePort     []int    `json:"source_port,omitempty"`
	ProcessName    []string `json:"process_name,omitempty"`
	ProcessPath    []string `json:"process_path,omitempty"`
	Network        []string `json:"network,omitempty"`
	Protocol       []string `json:"protocol,omitempty"`
	RuleSet        []string `json:"rule_set,omitempty"`
	Inbound        []string `json:"inbound,omitempty"`
}
