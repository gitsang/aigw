/**
 * Upstream API protocols shared by providers, provider models and virtual model
 * targets, in the canonical order the gateway uses when it has to fall back
 * from the inbound protocol to the first protocol the upstream supports.
 */
export const API_PROTOCOLS = [
  { value: 'chat.completions', label: 'Chat Completions', defaultEndpoint: '/v1/chat/completions' },
  { value: 'responses', label: 'Responses', defaultEndpoint: '/v1/responses' },
  { value: 'messages', label: 'Messages', defaultEndpoint: '/v1/messages' },
  {
    value: 'interactions',
    label: 'Interactions',
    defaultEndpoint: '/v1beta/interactions'
  }
];

export const API_PROTOCOL_VALUES = API_PROTOCOLS.map((protocol) => protocol.value);

/** Human-friendly label for an API protocol value. */
export function apiSchemaLabel(value) {
  return API_PROTOCOLS.find((protocol) => protocol.value === value)?.label ?? value;
}

/** Built-in upstream endpoint used when no explicit endpoint is configured. */
export function apiSchemaDefaultEndpoint(value) {
  return API_PROTOCOLS.find((protocol) => protocol.value === value)?.defaultEndpoint ?? '';
}

/** A stored api_schema counts as configured when it has at least one entry. */
function hasEntries(schema) {
  return !!schema && typeof schema === 'object' && Object.keys(schema).length > 0;
}

/** Blank editor value: every protocol enabled with the built-in endpoint. */
export function blankAPISchema() {
  return Object.fromEntries(
    API_PROTOCOLS.map((protocol) => [protocol.value, { enabled: true, endpoint: '' }])
  );
}

/**
 * Read a stored api_schema into the editor shape. An absent or empty schema
 * means "not configured": every protocol is enabled with the built-in endpoint.
 * A non-empty schema is the complete list of supported protocols, so any
 * missing entry is treated as disabled.
 */
export function apiSchemaToForm(schema) {
  if (!hasEntries(schema)) return blankAPISchema();
  return Object.fromEntries(
    API_PROTOCOLS.map((protocol) => [
      protocol.value,
      {
        enabled: Boolean(schema[protocol.value]?.enabled),
        endpoint: schema[protocol.value]?.endpoint ?? ''
      }
    ])
  );
}

/**
 * Flatten the editor value back into the admin API payload: always a complete
 * three-entry schema with trimmed endpoints.
 */
export function formToAPISchema(formSchema) {
  return Object.fromEntries(
    API_PROTOCOLS.map((protocol) => [
      protocol.value,
      {
        enabled: Boolean(formSchema?.[protocol.value]?.enabled),
        endpoint: String(formSchema?.[protocol.value]?.endpoint ?? '').trim()
      }
    ])
  );
}

/**
 * Console-side mirror of the gateway resolution, used only for previews. The
 * authoritative logic lives in Go (internal/store/catalog.go); keep the two in
 * sync when the resolution rules change.
 *
 * Support comes from the first non-empty schema (model, then provider), with an
 * entirely absent configuration enabling every protocol. The endpoint is
 * resolved per protocol: model -> provider -> built-in default. Disabled
 * protocols report an empty endpoint.
 */
export function effectiveAPISchema(modelSchema, providerSchema) {
  const supportSchema = hasEntries(modelSchema)
    ? modelSchema
    : hasEntries(providerSchema)
      ? providerSchema
      : null;
  const resolved = {};
  for (const protocol of API_PROTOCOLS) {
    const enabled = supportSchema ? Boolean(supportSchema[protocol.value]?.enabled) : true;
    let endpoint = '';
    if (enabled) {
      endpoint =
        String(modelSchema?.[protocol.value]?.endpoint ?? '').trim() ||
        String(providerSchema?.[protocol.value]?.endpoint ?? '').trim() ||
        protocol.defaultEndpoint;
    }
    resolved[protocol.value] = { enabled, endpoint };
  }
  return resolved;
}

/** Enabled protocol values, in canonical order. */
export function apiSchemaProtocols(schema) {
  return API_PROTOCOLS.filter((protocol) => schema?.[protocol.value]?.enabled).map(
    (protocol) => protocol.value
  );
}
