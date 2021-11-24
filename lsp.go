package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func counter() func() int32 {
	var n int32
	return func() int32 {
		n++
		return n
	}
}

type Lsp struct {
	name       string
	rootUri    protocol.URI
	fileUri    protocol.URI
	languageID protocol.LanguageIdentifier
}

var (
	lsp         Lsp
	version     = counter()
	idNum       = counter()
	stdin       io.WriteCloser
	stdoutRdr   *bufio.Reader
	diagnostics = make(chan []protocol.Diagnostic)
	quit        = make(chan struct{})
	logFile     *os.File
	requestType = make(map[jsonrpc2.ID]string)
)

func launchLsp(lspName string) {
	var cmd *exec.Cmd
	switch lspName {
	case "gopls":
		lsp.name = "gopls"
		lsp.rootUri = "file:///home/slzatz/go_fragments"
		lsp.fileUri = "file:///home/slzatz/go_fragments/main.go"
		lsp.languageID = "go"
		cmd = exec.Command("gopls", "serve", "-rpc.trace", "-logfile", "/home/slzatz/gopls_log")
	case "clangd":
		lsp.name = "clangd"
		lsp.rootUri = "file:///home/slzatz/clangd_examples"
		lsp.fileUri = "file:///home/slzatz/clangd_examples/test.cpp"
		lsp.languageID = "cpp"
		cmd = exec.Command("clangd", "--log=verbose")
		logFile, _ := os.Create("/home/slzatz/clangd_log")
		cmd.Stderr = logFile
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sess.showOrgMessage("Failed to create stdout pipe: %v", err)
		return
	}
	stdin, err = cmd.StdinPipe()
	if err != nil {
		sess.showOrgMessage("Failed to launch LSP: %v", err)
		return
	}
	err = cmd.Start()
	if err != nil {
		sess.showOrgMessage("Failed to start LSP: %v", err)
		return
	}
	stdoutRdr = bufio.NewReaderSize(stdout, 10000)

	go readMessages()

	//Client sends initialize method and server replies with result: Capabilities ...)
	params := protocol.InitializeParams{
		ProcessID:    0,
		RootURI:      lsp.rootUri,
		Capabilities: clientcapabilities,
	}
	sendRequest("initialize", params)

	sendNotification("initialized", struct{}{})

	// Client sends notification method:did/Open and
	textParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        lsp.fileUri,
			LanguageID: lsp.languageID,
			Text:       " ",
			Version:    version(),
			//Version:    1,
		},
	}
	sendNotification("textDocument/didOpen", textParams)

	// draining off any diagnostics before issuing didChange
	timeout := time.After(time.Second) //2
L:
	for {
		select {
		case <-diagnostics:
		case <-timeout:
			break L
		default:
		}
	}

	sess.showOrgMessage("LSP %s launched", lsp.name)
}

func shutdownLsp() {
	// tell server the file is closed
	closeParams := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: lsp.fileUri,
		},
	}
	sendNotification("textDocument/didClose", closeParams)

	// shutdown request sent to server
	sendRequest("shutdown", nil)

	// exit notification sent to server
	sendNotification("exit", nil)

	if lsp.name == "clangd" {
		logFile.Close()
		//quit <- struct{}{}
	} else {
		// this blocks for clangd so readMessages go routine doesn't terminate
		quit <- struct{}{}
	}
	sess.showOrgMessage("Shutdown LSP")

	lsp.name = ""
	lsp.rootUri = ""
	lsp.fileUri = ""
}

func sendDidChangeNotification(text string) {

	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Version: version()},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: text}},
	}
	sendNotification("textDocument/didChange", params)
}

func sendCompletionRequest(line, character uint32) {

	progressToken := protocol.NewProgressToken("test")
	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: progressToken},
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: progressToken},
		Context: nil,
	}
	sendRequest("textDocument/completion", params)
}

func sendHoverRequest(line, character uint32) {
	progressToken := protocol.NewProgressToken("test")
	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: progressToken},
	}
	sendRequest("textDocument/hover", params)
}

func sendSignatureHelpRequest(line, character uint32) {
	progressToken := protocol.NewProgressToken("test")
	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: progressToken},
		Context: nil,
	}
	sendRequest("textDocument/signatureHelp", params)
}

func sendRenameRequest(line, character uint32, newName string) {
	progressToken := protocol.NewProgressToken("test")
	params := protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: progressToken},
		NewName: newName,
	}
	sendRequest("textDocument/rename", params)
}

func sendDocumentHighlightRequest(line, character uint32) {
	progressToken := protocol.NewProgressToken("test")
	params := protocol.DocumentHighlightParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: progressToken},
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: progressToken},
	}
	sendRequest("textDocument/documentHighlight", params)
}

func sendDefinitionRequest(line, character uint32) {
	progressToken := protocol.NewProgressToken("test")
	params := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: progressToken},
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: progressToken},
	}
	sendRequest("textDocument/definition", params)
}

func sendReferenceRequest(line, character uint32) {
	progressToken := protocol.NewProgressToken("test")
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: lsp.fileUri},
			Position: protocol.Position{
				Line:      line,
				Character: character}},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: progressToken},
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: progressToken},
	}
	sendRequest("textDocument/references", params)
}
func readMessages() {
	var length int64
	name := lsp.name
	for {
		select {
		default:
			line, err := stdoutRdr.ReadString('\n')
			if err == io.EOF {
				// clangd never gets <-quit
				// but if you launch another lsp this is triggered
				sess.showEdMessage("ReadMessages(%s): Got EOF", name)
				return
			}
			if err != nil {
				sess.showEdMessage("ReadMessages: %s-%v", string(line), err)
			}

			if line == "" {
				continue
			}

			colon := strings.IndexRune(line, ':')
			if colon < 0 {
				continue
			}

			value := strings.TrimSpace(line[colon+1:])

			if length, err = strconv.ParseInt(value, 10, 32); err != nil {
				continue
			}

			if length <= 0 {
				continue
			}

			// to read the last two chars of '\r\n\r\n'
			line, err = stdoutRdr.ReadString('\n')
			if err != nil {
				sess.showEdMessage("ReadMessages: %s-%v", string(line), err)
			}

			data := make([]byte, length)

			if _, err = io.ReadFull(stdoutRdr, data); err != nil {
				continue
			}

			msg, err := jsonrpc2.DecodeMessage(data)
			switch msg := msg.(type) {

			case jsonrpc2.Request:
				// I don't think server can send a request to client??
				//if call, ok := msg.(*jsonrpc2.Call); ok
				notification := msg.(*jsonrpc2.Notification)
				notification.UnmarshalJSON(data)
				if notification.Method() == "textDocument/publishDiagnostics" {
					var params protocol.PublishDiagnosticsParams
					err := json.Unmarshal(notification.Params(), &params)
					if err != nil {
						sess.showEdMessage("Error unmarshaling diagnostics: %v", err)
						return
					}
					diagnostics <- params.Diagnostics
				}
			case *jsonrpc2.Response:
				msg.UnmarshalJSON(data)
				id := msg.ID()
				result := msg.Result()
				if result == nil {
					sess.showEdMessage("Got null/nil result for %s", requestType[id])
					continue
				}

				switch requestType[id] {
				case "initialize", "shutdown":
					continue
				case "textDocument/completion":
					var completion protocol.CompletionList
					err := json.Unmarshal(result, &completion)
					if err != nil {
						sess.showEdMessage("Completion Error: %v", err)
					}
					p.drawCompletionItems(completion)
				case "textDocument/hover":
					var hover protocol.Hover
					err := json.Unmarshal(result, &hover)
					if err != nil {
						sess.showEdMessage("hover error: %v", err)
					}
					p.drawHover(hover)
				case "textDocument/signatureHelp":
					var signature protocol.SignatureHelp
					err := json.Unmarshal(result, &signature)
					if err != nil {
						sess.showEdMessage("signatureHelp error: %v", err)
					}
					p.drawSignatureHelp(signature)
				case "textDocument/rename":
					var workspaceEdit protocol.WorkspaceEdit
					err := json.Unmarshal(result, &workspaceEdit)
					if err != nil {
						sess.showEdMessage("rename error: %v", err)
					}
					p.applyWorkspaceEdit(workspaceEdit)
				case "textDocument/documentHighlight":
					var documentHighlight []protocol.DocumentHighlight
					err := json.Unmarshal(result, &documentHighlight)
					if err != nil {
						sess.showEdMessage("documentHighlight error: %v", err)
					}
					p.drawDocumentHighlight(documentHighlight)
				case "textDocument/definition":
					//var definition []protocol.LocationLink
					var definition []protocol.Location
					err := json.Unmarshal(result, &definition)
					if err != nil {
						sess.showEdMessage("definition error: %v", err)
					}
					p.drawDefinition(definition)
				case "textDocument/references":
					//var definition []protocol.LocationLink
					var references []protocol.Location
					err := json.Unmarshal(result, &references)
					if err != nil {
						sess.showEdMessage("references error: %v", err)
					}
					p.drawReference(references)
				}
			}
		case <-quit: //clangd never gets here; gopls does
			return
		}
	}
}

func send(msg json.Marshaler) {
	b, err := msg.MarshalJSON()
	if err != nil {
		sess.showEdMessage("Error sending to server: %v", err)
		return
	}
	s := string(b)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(s))
	s = header + s

	io.WriteString(stdin, s)
}

func sendRequest(method string, params interface{}) {
	id := jsonrpc2.NewNumberID(idNum())
	requestType[id] = method
	request, err := jsonrpc2.NewCall(id, method, params)
	if err != nil {
		sess.showEdMessage("Error creating new request: %v", err)
		return
	}
	send(request)
}

func sendNotification(method string, params interface{}) {
	notify, err := jsonrpc2.NewNotification(method, params)
	if err != nil {
		sess.showEdMessage("Error creating new notification: %v", err)
		return
	}
	send(notify)
}
