package alexa

// AlexaRequest is the top-level Alexa Skills Kit request envelope.
type AlexaRequest struct {
	Version string   `json:"version"`
	Session *Session `json:"session"`
	Context *Context `json:"context,omitempty"`
	Request *Request `json:"request"`
}

type Session struct {
	New         bool                   `json:"new"`
	SessionID   string                 `json:"sessionId"`
	Application Application            `json:"application"`
	User        User                   `json:"user"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

type Application struct {
	ApplicationID string `json:"applicationId"`
}

type User struct {
	UserID      string `json:"userId"`
	AccessToken string `json:"accessToken,omitempty"`
}

type Context struct {
	System SystemContext `json:"System"`
}

type SystemContext struct {
	Application Application `json:"application"`
	User        User        `json:"user"`
	Device      *Device     `json:"device,omitempty"`
	APIEndpoint string      `json:"apiEndpoint,omitempty"`
}

type Device struct {
	DeviceID string `json:"deviceId,omitempty"`
}

type Request struct {
	Type      string  `json:"type"`
	RequestID string  `json:"requestId"`
	Timestamp string  `json:"timestamp"`
	Locale    string  `json:"locale"`
	Intent    *Intent `json:"intent,omitempty"`
	Reason    string  `json:"reason,omitempty"` // SessionEndedRequest
}

type Intent struct {
	Name               string          `json:"name"`
	ConfirmationStatus string          `json:"confirmationStatus,omitempty"`
	Slots              map[string]Slot `json:"slots,omitempty"`
}

type Slot struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	ConfirmationStatus string `json:"confirmationStatus,omitempty"`
}

// AlexaResponse is the top-level Alexa Skills Kit response envelope.
type AlexaResponse struct {
	Version           string                 `json:"version"`
	SessionAttributes map[string]interface{} `json:"sessionAttributes,omitempty"`
	Response          ResponseBody           `json:"response"`
}

type ResponseBody struct {
	OutputSpeech     *OutputSpeech `json:"outputSpeech,omitempty"`
	Card             *Card         `json:"card,omitempty"`
	Reprompt         *Reprompt     `json:"reprompt,omitempty"`
	ShouldEndSession *bool         `json:"shouldEndSession,omitempty"`
}

type OutputSpeech struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	SSML string `json:"ssml,omitempty"`
}

type Card struct {
	Type    string `json:"type"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
	Text    string `json:"text,omitempty"`
}

type Reprompt struct {
	OutputSpeech OutputSpeech `json:"outputSpeech"`
}
