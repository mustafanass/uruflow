const labels: Record<string, string> = {
  'intended placement': 'Assigned runner',
  'observed placement': 'Container instance',
  'unexpected observation': 'Not in current config',
  'declared service': 'Configured service',
  'authoritative project': 'Project configuration',
  'outbound authenticated UFP link': 'UFP agent connection',
  'observed attachment': 'Observed attachment',
  'desired attachment': 'Configured attachment',
  'release artifact (latest attempt)': 'Latest release artifact',
}
export function resourceLabel(value: string) {
  return labels[value] || value
}
