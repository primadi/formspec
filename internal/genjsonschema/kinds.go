package genjsonschema

// KindMapping returns the mapping from YAML kind values to Go spec struct names.
func KindMapping() []KindEntry {
	return []KindEntry{
		// — Core Basic —
		{Kind: "App", SpecStruct: "AppSpec"},
		{Kind: "Module", SpecStruct: "ModuleSpec"},
		{Kind: "Document", SpecStruct: "DocumentSpec"},
		{Kind: "Entity", SpecStruct: "DocumentSpec", Deprecated: true, Aliases: []string{"Document"}},
		{Kind: "Service", SpecStruct: "ServiceSpec"},
		{Kind: "Config", SpecStruct: "ConfigSpec"},
		{Kind: "Migration", SpecStruct: "MigrationSpec"},
		{Kind: "Subscription", SpecStruct: "SubscriptionSpec"},

		// — Core Extended —
		{Kind: "Workflow", SpecStruct: "WorkflowSpec"},
		{Kind: "Api", SpecStruct: "ApiSpec"},
		{Kind: "Webhook", SpecStruct: "WebhookSpec"},
		{Kind: "Integrator", SpecStruct: "IntegratorSpec"},
		{Kind: "Mockup", SpecStruct: "MockupSpec"},
		{Kind: "KindDefinition", SpecStruct: "KindDefinitionSpec"},

		// — Control Plane —
		{Kind: "Environment", SpecStruct: "EnvironmentSpec"},
		{Kind: "Policy", SpecStruct: "PolicySpec"},
		{Kind: "Datastore", SpecStruct: "DatastoreSpec"},

		// — Renderer Kinds —
		{Kind: "Renderer", SpecStruct: "RendererSpec"},
		{Kind: "PersistBackend", SpecStruct: "PersistBackendSpec"},

		// — Frontend Kinds —
		{Kind: "Page", SpecStruct: "PageSpec"},
		{Kind: "Form", SpecStruct: "FormSpec"},
		{Kind: "Table", SpecStruct: "TableSpec"},
		{Kind: "Dashboard", SpecStruct: "DashboardSpec"},
		{Kind: "Widget", SpecStruct: "WidgetSpec"},
		{Kind: "Report", SpecStruct: "ReportSpec"},
		{Kind: "Wizard", SpecStruct: "WizardSpec"},
		{Kind: "Kanban", SpecStruct: "KanbanSpec"},
		{Kind: "Timeline", SpecStruct: "TimelineSpec"},
		{Kind: "Print", SpecStruct: "PrintSpec"},
		{Kind: "Theme", SpecStruct: "ThemeSpec"},
		{Kind: "Calendar", SpecStruct: "CalendarSpec"},
		{Kind: "Listing", SpecStruct: "ListingSpec"},
		{Kind: "ApprovalInbox", SpecStruct: "ApprovalInboxSpec"},
		{Kind: "NotificationCenter", SpecStruct: "NotificationCenterSpec"},

		// — Visual Spec Kinds (meta-kind) —
		{Kind: "VisualSpecKind", SpecStruct: "VisualSpecKindSpec"},
	}
}
