package printer

// PrinterInfo holds information about the configured printer.
type PrinterInfo struct {
	Name         string            `json:"name"`
	IP           string            `json:"ip"`
	MakeModel    string            `json:"make_model"`
	State        string            `json:"state"`
	StateReasons []string          `json:"state_reasons,omitempty"`
	Location     string            `json:"location,omitempty"`
	Info         string            `json:"info,omitempty"`
	URIs         map[string]string `json:"uris,omitempty"`
	Capabilities *Capabilities     `json:"capabilities,omitempty"`
}

type Capabilities struct {
	Color           bool     `json:"color"`
	Duplex          bool     `json:"duplex"`
	AutoCut         bool     `json:"auto_cut"`
	TapeWidths      []string `json:"tape_widths,omitempty"`
	MediaTypes      []string `json:"media_types,omitempty"`
	Resolutions     []string `json:"resolutions,omitempty"`
	DocumentFormats []string `json:"document_formats,omitempty"`
}

type SupplyLevel struct {
	Name      string `json:"name"`
	Level     int    `json:"level"`
	MaxLevel  int    `json:"max_level"`
	Color     string `json:"color,omitempty"`
	Type      string `json:"type,omitempty"`
}

type SupplyStatus struct {
	PrinterName string        `json:"printer_name"`
	Supplies    []SupplyLevel `json:"supplies"`
	SNMPSuccess bool          `json:"snmp_success"`
	Error       string        `json:"error,omitempty"`
}

type PrintJob struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	State      string `json:"state"`
	Owner      string `json:"owner,omitempty"`
	Size       int    `json:"size,omitempty"`
	Pages      int    `json:"pages,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type PrintResult struct {
	JobID   int    `json:"job_id,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ConnectivityResult struct {
	PrinterIP     string `json:"printer_ip"`
	IPPReachable  bool   `json:"ipp_reachable"`
	SNMPReachable bool   `json:"snmp_reachable"`
	HTTPReachable bool   `json:"http_reachable"`
	Error         string `json:"error,omitempty"`
}
