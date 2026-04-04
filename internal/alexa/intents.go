package alexa

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"go-mcp-brother-label-printer-direct/internal/printer"
)

func (h *Handler) handleGetPrinterStatus() *AlexaResponse {
	info, err := h.ippClient.GetPrinterInfo()
	if err != nil {
		slog.Error("alexa: get printer info failed", "error", err)
		return plainTextResponse("I couldn't reach the printer. It may be offline.", true)
	}

	speech := formatPrinterStatus(info)

	var cardLines []string
	cardLines = append(cardLines, fmt.Sprintf("Model: %s", info.MakeModel))
	cardLines = append(cardLines, fmt.Sprintf("IP: %s", info.IP))
	cardLines = append(cardLines, fmt.Sprintf("State: %s", info.State))
	if info.Capabilities != nil {
		if info.Capabilities.Color {
			cardLines = append(cardLines, "Color: Yes")
		}
		if info.Capabilities.Duplex {
			cardLines = append(cardLines, "Duplex: Yes")
		}
	}

	return cardResponse("Printer Status", strings.Join(cardLines, "\n"), speech, true)
}

func (h *Handler) handleGetInkLevels() *AlexaResponse {
	status, err := h.snmpClient.GetSupplyLevels()
	if err != nil {
		slog.Error("alexa: get supply levels failed", "error", err)
		return plainTextResponse("I couldn't retrieve the ink levels. The printer may be offline.", true)
	}

	speech := formatSupplyLevels(status)

	var cardLines []string
	for _, s := range status.Supplies {
		label := s.Color
		if label == "" {
			label = s.Name
		}
		pct := s.Level
		if s.MaxLevel > 0 && s.MaxLevel != 100 {
			pct = (s.Level * 100) / s.MaxLevel
		}
		cardLines = append(cardLines, fmt.Sprintf("%s: %d%%", label, pct))
	}

	return cardResponse("Ink/Toner Levels", strings.Join(cardLines, "\n"), speech, true)
}

func (h *Handler) handlePrintText(req *AlexaRequest) *AlexaResponse {
	text := slotValue(req, "text")
	if text == "" {
		return plainTextResponse("What would you like me to print?", false)
	}

	// Check for pending confirmation
	attrs := sessionAttrs(req)
	if attrs["pending_action"] == "print_text" && attrs["pending_text"] == text {
		// Already asked for confirmation, this shouldn't happen via this path
		return h.executePrintText(text)
	}

	// Ask for confirmation
	attrs["pending_action"] = "print_text"
	attrs["pending_text"] = text
	speech := fmt.Sprintf("I'll print '%s'. Should I go ahead?", text)
	return confirmResponse(speech, attrs)
}

func (h *Handler) executePrintText(text string) *AlexaResponse {
	result, err := h.ippClient.PrintText(text, "Alexa Print", 1)
	if err != nil {
		slog.Error("alexa: print text failed", "error", err)
		return plainTextResponse("The print job failed. Please try again.", true)
	}

	speech := fmt.Sprintf("Print job %d has been submitted successfully.", result.JobID)
	cardText := fmt.Sprintf("Job ID: %d\nStatus: %s\nContent: %s", result.JobID, result.Status, text)
	return cardResponse("Print Job Submitted", cardText, speech, true)
}

func (h *Handler) handleGetPrintQueue() *AlexaResponse {
	jobs, err := h.ippClient.GetJobs()
	if err != nil {
		slog.Error("alexa: get print queue failed", "error", err)
		return plainTextResponse("I couldn't retrieve the print queue.", true)
	}

	speech := formatPrintQueue(jobs)

	var cardLines []string
	for _, job := range jobs {
		cardLines = append(cardLines, fmt.Sprintf("Job %d: %s (%s)", job.ID, job.Name, job.State))
	}
	cardText := "No jobs in queue"
	if len(cardLines) > 0 {
		cardText = strings.Join(cardLines, "\n")
	}

	return cardResponse("Print Queue", cardText, speech, true)
}

func (h *Handler) handleGetJobStatus(req *AlexaRequest) *AlexaResponse {
	jobIDStr := slotValue(req, "jobId")
	if jobIDStr == "" {
		return plainTextResponse("Which job number would you like to check?", false)
	}

	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		return plainTextResponse("I didn't understand that job number. Please say a number.", false)
	}

	job, err := h.ippClient.GetJobStatus(jobID)
	if err != nil {
		slog.Error("alexa: get job status failed", "error", err, "job_id", jobID)
		return plainTextResponse(fmt.Sprintf("I couldn't find job %d.", jobID), true)
	}

	speech := formatJobStatus(job)
	cardText := fmt.Sprintf("Job ID: %d\nName: %s\nState: %s", job.ID, job.Name, job.State)
	return cardResponse("Job Status", cardText, speech, true)
}

func (h *Handler) handleCancelJob(req *AlexaRequest) *AlexaResponse {
	jobIDStr := slotValue(req, "jobId")
	if jobIDStr == "" {
		return plainTextResponse("Which job number would you like to cancel?", false)
	}

	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		return plainTextResponse("I didn't understand that job number. Please say a number.", false)
	}

	// Ask for confirmation
	attrs := sessionAttrs(req)
	if attrs["pending_action"] == "cancel_job" && attrs["pending_job_id"] == jobIDStr {
		return h.executeCancelJob(jobID)
	}

	attrs["pending_action"] = "cancel_job"
	attrs["pending_job_id"] = jobIDStr
	speech := fmt.Sprintf("I'll cancel job %d. Should I go ahead?", jobID)
	return confirmResponse(speech, attrs)
}

func (h *Handler) executeCancelJob(jobID int) *AlexaResponse {
	if err := h.ippClient.CancelJob(jobID); err != nil {
		slog.Error("alexa: cancel job failed", "error", err, "job_id", jobID)
		return plainTextResponse(fmt.Sprintf("I couldn't cancel job %d.", jobID), true)
	}

	speech := fmt.Sprintf("Job %d has been cancelled.", jobID)
	return cardResponse("Job Cancelled", fmt.Sprintf("Job %d cancelled successfully", jobID), speech, true)
}

func (h *Handler) handleTestConnectivity() *AlexaResponse {
	result := printer.TestConnectivity(h.printerIP, h.dialFunc)
	speech := formatConnectivity(result)

	var cardLines []string
	cardLines = append(cardLines, fmt.Sprintf("Printer: %s", result.PrinterIP))
	cardLines = append(cardLines, fmt.Sprintf("IPP (631): %s", boolToStatus(result.IPPReachable)))
	cardLines = append(cardLines, fmt.Sprintf("SNMP (161): %s", boolToStatus(result.SNMPReachable)))
	cardLines = append(cardLines, fmt.Sprintf("HTTP (80): %s", boolToStatus(result.HTTPReachable)))

	return cardResponse("Connectivity Test", strings.Join(cardLines, "\n"), speech, true)
}

// Confirmation flow handlers

func (h *Handler) handleConfirmYes(req *AlexaRequest) *AlexaResponse {
	attrs := sessionAttrs(req)
	action, _ := attrs["pending_action"].(string)

	switch action {
	case "print_text":
		text, _ := attrs["pending_text"].(string)
		if text == "" {
			return plainTextResponse("I'm not sure what to print. Please try again.", true)
		}
		return h.executePrintText(text)

	case "cancel_job":
		jobIDStr, _ := attrs["pending_job_id"].(string)
		jobID, err := strconv.Atoi(jobIDStr)
		if err != nil {
			return plainTextResponse("I lost track of which job to cancel. Please try again.", true)
		}
		return h.executeCancelJob(jobID)

	default:
		return plainTextResponse("I'm not sure what you're confirming. Please try again.", true)
	}
}

func (h *Handler) handleConfirmNo() *AlexaResponse {
	return plainTextResponse("Okay, cancelled. Is there anything else?", false)
}

// Helpers

func slotValue(req *AlexaRequest, name string) string {
	if req.Request == nil || req.Request.Intent == nil {
		return ""
	}
	slot, ok := req.Request.Intent.Slots[name]
	if !ok {
		return ""
	}
	return slot.Value
}

func sessionAttrs(req *AlexaRequest) map[string]interface{} {
	if req.Session != nil && req.Session.Attributes != nil {
		return req.Session.Attributes
	}
	return make(map[string]interface{})
}

func boolToStatus(b bool) string {
	if b {
		return "OK"
	}
	return "UNREACHABLE"
}
