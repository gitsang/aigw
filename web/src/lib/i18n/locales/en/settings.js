/** Settings page. */
export default {
  title: 'Settings · aigw',
  heading: 'Settings',
  description: 'Gateway-wide runtime configuration. Changes apply immediately, no restart needed.',
  autoMatch: {
    title: 'Automatic model matching',
    description: 'Controls what the gateway does when a requested model has no virtual model.',
    label: 'Automatic model matching',
    on: 'On',
    off: 'Off',
    body: 'When enabled, a requested model name without a matching virtual model is resolved to the first provider model whose public id contains that name (public ids look like provider/model), and the request is forwarded to that upstream directly.',
    enabledToast: 'Automatic model matching enabled',
    disabledToast: 'Automatic model matching disabled'
  },
  example: {
    title: 'Matching example',
    requestModel: 'For a request model',
    withoutVirtualModel: 'with no virtual model of the same name:',
    step1: '1. Walk the enabled provider models in order;',
    step2: '2. Take the first model whose public id contains that string, e.g.',
    step2Tail: ';',
    step3: "3. Send the upstream request with that model's protocol, path, retry and proxy settings."
  },
  note: 'Existing virtual models always win; automatic matching is only a fallback and never writes a virtual model. The request log records ',
  noteKey: ' as ',
  noteTail: ' for auditing.'
};
