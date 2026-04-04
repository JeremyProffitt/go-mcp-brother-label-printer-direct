package alexa

import (
	"fmt"
	"strings"

	"go-mcp-brother-label-printer-direct/internal/printer"
)

func formatPrinterStatus(info *printer.PrinterInfo) string {
	state := info.State
	switch state {
	case "idle":
		state = "idle and ready to print"
	case "processing":
		state = "currently processing a job"
	case "stopped":
		state = "stopped"
	}

	speech := fmt.Sprintf("Your %s is %s.", info.MakeModel, state)

	if len(info.StateReasons) > 0 {
		reasons := strings.Join(info.StateReasons, ", ")
		if reasons != "none" {
			speech += fmt.Sprintf(" Status details: %s.", reasons)
		}
	}

	return speech
}

func formatSupplyLevels(status *printer.SupplyStatus) string {
	if len(status.Supplies) == 0 {
		return "I couldn't retrieve any supply level information."
	}

	var parts []string
	var lowWarnings []string

	for _, s := range status.Supplies {
		label := s.Color
		if label == "" {
			label = s.Name
		}

		pct := s.Level
		if s.MaxLevel > 0 && s.MaxLevel != 100 {
			pct = (s.Level * 100) / s.MaxLevel
		}

		parts = append(parts, fmt.Sprintf("%s at %d percent", label, pct))

		if pct < 10 {
			lowWarnings = append(lowWarnings, fmt.Sprintf("%s is critically low at %d percent", label, pct))
		} else if pct < 20 {
			lowWarnings = append(lowWarnings, fmt.Sprintf("%s is low at %d percent", label, pct))
		}
	}

	speech := "Your toner levels are: " + joinSpeechList(parts) + "."

	if len(lowWarnings) > 0 {
		speech += " Warning: " + joinSpeechList(lowWarnings) + "."
	}

	return speech
}

func formatPrintQueue(jobs []printer.PrintJob) string {
	if len(jobs) == 0 {
		return "Your print queue is empty. There are no pending jobs."
	}

	speech := fmt.Sprintf("You have %d job", len(jobs))
	if len(jobs) != 1 {
		speech += "s"
	}
	speech += " in the queue. "

	for i, job := range jobs {
		if i >= 5 {
			speech += fmt.Sprintf("And %d more.", len(jobs)-5)
			break
		}
		speech += fmt.Sprintf("Job %d, %s, status %s. ", job.ID, job.Name, job.State)
	}

	return speech
}

func formatJobStatus(job *printer.PrintJob) string {
	return fmt.Sprintf("Job %d, %s, is %s.", job.ID, job.Name, job.State)
}

func formatConnectivity(result *printer.ConnectivityResult) string {
	reachable := 0
	total := 3
	var issues []string

	if result.IPPReachable {
		reachable++
	} else {
		issues = append(issues, "IPP on port 631")
	}
	if result.SNMPReachable {
		reachable++
	} else {
		issues = append(issues, "SNMP on port 161")
	}
	if result.HTTPReachable {
		reachable++
	} else {
		issues = append(issues, "HTTP on port 80")
	}

	if reachable == total {
		return fmt.Sprintf("The printer at %s is fully reachable on all %d ports.", result.PrinterIP, total)
	}

	if reachable == 0 {
		return fmt.Sprintf("The printer at %s is not reachable on any port. It may be offline or unreachable.", result.PrinterIP)
	}

	return fmt.Sprintf("The printer at %s is reachable on %d of %d ports. Unreachable: %s.",
		result.PrinterIP, reachable, total, joinSpeechList(issues))
}

// joinSpeechList joins items with commas and "and" before the last item.
func joinSpeechList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

// Response builders

func boolPtr(b bool) *bool {
	return &b
}

func plainTextResponse(text string, endSession bool) *AlexaResponse {
	return &AlexaResponse{
		Version: "1.0",
		Response: ResponseBody{
			OutputSpeech: &OutputSpeech{
				Type: "PlainText",
				Text: text,
			},
			ShouldEndSession: boolPtr(endSession),
		},
	}
}

func cardResponse(title, cardText, speech string, endSession bool) *AlexaResponse {
	return &AlexaResponse{
		Version: "1.0",
		Response: ResponseBody{
			OutputSpeech: &OutputSpeech{
				Type: "PlainText",
				Text: speech,
			},
			Card: &Card{
				Type:    "Simple",
				Title:   title,
				Content: cardText,
			},
			ShouldEndSession: boolPtr(endSession),
		},
	}
}

func confirmResponse(speech string, sessionAttrs map[string]interface{}) *AlexaResponse {
	return &AlexaResponse{
		Version:           "1.0",
		SessionAttributes: sessionAttrs,
		Response: ResponseBody{
			OutputSpeech: &OutputSpeech{
				Type: "PlainText",
				Text: speech,
			},
			Reprompt: &Reprompt{
				OutputSpeech: OutputSpeech{
					Type: "PlainText",
					Text: "Should I go ahead? Say yes or no.",
				},
			},
			ShouldEndSession: boolPtr(false),
		},
	}
}

func linkAccountResponse() *AlexaResponse {
	return &AlexaResponse{
		Version: "1.0",
		Response: ResponseBody{
			OutputSpeech: &OutputSpeech{
				Type: "PlainText",
				Text: "Please link your account in the Alexa app to use this skill.",
			},
			Card: &Card{
				Type: "LinkAccount",
			},
			ShouldEndSession: boolPtr(true),
		},
	}
}
