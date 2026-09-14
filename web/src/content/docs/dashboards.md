# Building a dashboard

The editor composes panels around one synchronized investigation scope and persists definitions through the backend.

## Start from a template

Templates cover platform overview, ingestion health, distributed services, multi-agent operations, model efficiency, financial operations, and SLO review. A template is an editable definition, not a source of demo telemetry.

## Change representation

Each panel offers only visualizations compatible with its signal. Changing between line, distribution, tunnel, radar, pipeline, transmission, agent, finance, and topology views preserves the query and filters.

## Arrange and persist

Drag panels to reorder, change compact/wide sizing, duplicate, delete, or edit them. **Save dashboard** writes the configuration and an immutable version through `/api/dashboards`; presentation mode removes editing controls.

> Panel queries remain bounded. Large-result aggregation belongs on the server, not in thousands of browser SVG nodes.
