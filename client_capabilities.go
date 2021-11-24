package main

import "go.lsp.dev/protocol"

var clientcapabilities = protocol.ClientCapabilities{
	Workspace: &protocol.WorkspaceClientCapabilities{
		ApplyEdit: true,
		WorkspaceEdit: &protocol.WorkspaceClientCapabilitiesWorkspaceEdit{
			DocumentChanges:    true,
			FailureHandling:    "FailureHandling",
			ResourceOperations: []string{"ResourceOperations"},
		},
		DidChangeConfiguration: &protocol.DidChangeConfigurationWorkspaceClientCapabilities{
			DynamicRegistration: false,
		},
		DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesWorkspaceClientCapabilities{
			DynamicRegistration: false,
		},
		Symbol: &protocol.WorkspaceSymbolClientCapabilities{
			DynamicRegistration: false,
			SymbolKind: &protocol.SymbolKindCapabilities{
				ValueSet: []protocol.SymbolKind{
					protocol.SymbolKindFile,
					protocol.SymbolKindModule,
					protocol.SymbolKindNamespace,
					protocol.SymbolKindPackage,
					protocol.SymbolKindClass,
					protocol.SymbolKindMethod,
				},
			},
		},
		ExecuteCommand: &protocol.ExecuteCommandClientCapabilities{
			DynamicRegistration: false,
		},
		WorkspaceFolders: true,
		Configuration:    false,
	},
	TextDocument: &protocol.TextDocumentClientCapabilities{
		Synchronization: &protocol.TextDocumentSyncClientCapabilities{
			DynamicRegistration: false,
			WillSave:            true,
			WillSaveWaitUntil:   true,
			DidSave:             true,
		},
		Completion: &protocol.CompletionTextDocumentClientCapabilities{
			DynamicRegistration: false,
			CompletionItem: &protocol.CompletionTextDocumentClientCapabilitiesItem{
				SnippetSupport:          true,
				CommitCharactersSupport: true,
				DocumentationFormat: []protocol.MarkupKind{
					protocol.PlainText,
					protocol.Markdown,
				},
				DeprecatedSupport: true,
				PreselectSupport:  true,
			},
			CompletionItemKind: &protocol.CompletionTextDocumentClientCapabilitiesItemKind{
				ValueSet: []protocol.CompletionItemKind{protocol.CompletionItemKindText},
			},
			ContextSupport: true,
		},
		Hover: &protocol.HoverTextDocumentClientCapabilities{
			DynamicRegistration: false,
			ContentFormat: []protocol.MarkupKind{
				protocol.PlainText,
				protocol.Markdown,
			},
		},
		SignatureHelp: &protocol.SignatureHelpTextDocumentClientCapabilities{
			DynamicRegistration: false,
			SignatureInformation: &protocol.TextDocumentClientCapabilitiesSignatureInformation{
				DocumentationFormat: []protocol.MarkupKind{
					protocol.PlainText,
					protocol.Markdown,
				},
			},
		},
		Declaration: &protocol.DeclarationTextDocumentClientCapabilities{
			DynamicRegistration: false,
			LinkSupport:         true,
		},
		Definition: &protocol.DefinitionTextDocumentClientCapabilities{
			DynamicRegistration: false,
			LinkSupport:         true,
		},
		TypeDefinition: &protocol.TypeDefinitionTextDocumentClientCapabilities{
			DynamicRegistration: false,
			LinkSupport:         true,
		},
		Implementation: &protocol.ImplementationTextDocumentClientCapabilities{
			DynamicRegistration: false,
			LinkSupport:         true,
		},
		References: &protocol.ReferencesTextDocumentClientCapabilities{
			DynamicRegistration: false,
		},
		DocumentHighlight: &protocol.DocumentHighlightClientCapabilities{
			DynamicRegistration: false,
		},
		DocumentSymbol: &protocol.DocumentSymbolClientCapabilities{
			DynamicRegistration: false,
			SymbolKind: &protocol.SymbolKindCapabilities{
				ValueSet: []protocol.SymbolKind{
					protocol.SymbolKindFile,
					protocol.SymbolKindModule,
					protocol.SymbolKindNamespace,
					protocol.SymbolKindPackage,
					protocol.SymbolKindClass,
					protocol.SymbolKindMethod,
				},
			},
			HierarchicalDocumentSymbolSupport: true,
		},
		CodeAction: &protocol.CodeActionClientCapabilities{
			DynamicRegistration: false,
			CodeActionLiteralSupport: &protocol.CodeActionClientCapabilitiesLiteralSupport{
				CodeActionKind: &protocol.CodeActionClientCapabilitiesKind{
					ValueSet: []protocol.CodeActionKind{
						protocol.QuickFix,
						protocol.Refactor,
						protocol.RefactorExtract,
						protocol.RefactorRewrite,
						protocol.Source,
						protocol.SourceOrganizeImports,
					},
				},
			},
		},
		CodeLens: &protocol.CodeLensClientCapabilities{
			DynamicRegistration: false,
		},
		DocumentLink: &protocol.DocumentLinkClientCapabilities{
			DynamicRegistration: false,
		},
		ColorProvider: &protocol.DocumentColorClientCapabilities{
			DynamicRegistration: false,
		},
		Formatting: &protocol.DocumentFormattingClientCapabilities{
			DynamicRegistration: false,
		},
		RangeFormatting: &protocol.DocumentRangeFormattingClientCapabilities{
			DynamicRegistration: false,
		},
		OnTypeFormatting: &protocol.DocumentOnTypeFormattingClientCapabilities{
			DynamicRegistration: false,
		},
		PublishDiagnostics: &protocol.PublishDiagnosticsClientCapabilities{
			RelatedInformation: true,
		},
		Rename: &protocol.RenameClientCapabilities{
			DynamicRegistration: false,
			PrepareSupport:      true,
		},
		FoldingRange: &protocol.FoldingRangeClientCapabilities{
			DynamicRegistration: false,
			RangeLimit:          uint32(5),
			LineFoldingOnly:     true,
		},
		SelectionRange: &protocol.SelectionRangeClientCapabilities{
			DynamicRegistration: false,
		},
		CallHierarchy: &protocol.CallHierarchyClientCapabilities{
			DynamicRegistration: false,
		},
		SemanticTokens: &protocol.SemanticTokensClientCapabilities{
			DynamicRegistration: false,
			Requests: protocol.SemanticTokensWorkspaceClientCapabilitiesRequests{
				Range: true,
				Full:  true,
			},
			TokenTypes:     []string{"test", "tokenTypes"},
			TokenModifiers: []string{"test", "tokenModifiers"},
			Formats: []protocol.TokenFormat{
				protocol.TokenFormatRelative,
			},
			OverlappingTokenSupport: true,
			MultilineTokenSupport:   true,
		},
		LinkedEditingRange: &protocol.LinkedEditingRangeClientCapabilities{
			DynamicRegistration: false,
		},
		Moniker: &protocol.MonikerClientCapabilities{
			DynamicRegistration: false,
		},
	},
	Window: &protocol.WindowClientCapabilities{
		WorkDoneProgress: false,
		ShowMessage: &protocol.ShowMessageRequestClientCapabilities{
			MessageActionItem: &protocol.ShowMessageRequestClientCapabilitiesMessageActionItem{
				AdditionalPropertiesSupport: true,
			},
		},
		ShowDocument: &protocol.ShowDocumentClientCapabilities{
			Support: true,
		},
	},
	General: &protocol.GeneralClientCapabilities{
		RegularExpressions: &protocol.RegularExpressionsClientCapabilities{
			Engine:  "ECMAScript",
			Version: "ES2020",
		},
		Markdown: &protocol.MarkdownClientCapabilities{
			Parser:  "marked",
			Version: "1.1.0",
		},
	},
	Experimental: "testExperimental",
}
