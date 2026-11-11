import { useState, type ReactNode } from 'react'

import { KeyRound, Pencil, Power, ShieldCheck } from 'lucide-react'

import { api } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import type { WebhookSecret, WebhookSettings as Settings } from '@/shared/types'
import {
  Badge,
  Code,
  ErrorState,
  Facts,
  Modal,
  Panel,
  Spinner,
} from '@/shared/ui'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { time } from '@/shared/utils'

import { WebhookCertificate } from './WebhookCertificate'
import { WebhookListener } from './WebhookListener'

const url = '/api/settings/webhook'

export function WebhookSettings() {
  const { confirm } = useWorkspace()
  const { data, error, reload, setData } = useLoad<Settings>(url)
  const [editing, setEditing] = useState('')
  const [secret, setSecret] = useState('')
  if (error) return <ErrorState message={error} retry={reload} />
  if (!data) return <Spinner />
  const settings = data
  function toggle() {
    confirm({
      title: settings.enabled ? 'Stop receiving pushes' : 'Receive pushes',
      description: settings.enabled
        ? 'Git providers receive an error for every push until deliveries are turned back on. Project branch settings stay as they are.'
        : 'Signed pushes start releases again for every enabled project branch.',
      label: settings.enabled ? 'Stop deliveries' : 'Receive pushes',
      danger: settings.enabled,
      success: settings.enabled
        ? 'Push deliveries stopped'
        : 'Push deliveries enabled',
      run: async () =>
        setData(
          await api<Settings>(url, {
            method: 'PUT',
            body: {
              revision: settings.revision,
              enabled: !settings.enabled,
              host: settings.host,
              port: settings.port,
              path: settings.path,
              tls: settings.tls,
            },
          }),
        ),
    })
  }
  function regenerate() {
    confirm({
      title: 'Regenerate webhook secret',
      description:
        'The new secret is shown once. The current secret keeps working for 24 hours so every repository webhook can be updated.',
      label: 'Regenerate secret',
      success: 'Webhook secret regenerated',
      run: async () => {
        const result = await api<WebhookSecret>(url + '/secret', {
          method: 'POST',
          body: { revision: settings.revision },
        })
        setData(result.settings)
        setSecret(result.secret)
      },
    })
  }
  const facts: [string, ReactNode][] = [
    ['Status', <Badge value={settings.enabled ? 'enabled' : 'disabled'} />],
    ['Webhook URL', <Code value={settings.endpoint} copy />],
    [
      'Listener',
      `${settings.host || 'Server bind address'} · port ${settings.port} · ${settings.tls ? 'HTTPS' : 'HTTP'}`,
    ],
    [
      'Certificate',
      !settings.tls
        ? 'Not used over HTTP'
        : settings.certificate
          ? `${settings.certificate.subject || settings.certificate.names.join(', ') || 'Custom certificate'} · expires ${time(settings.certificate.expires_at)}`
          : 'Server certificate',
    ],
    [
      'Secret',
      <>
        Ending in <code>{settings.secret_hint}</code> · created{' '}
        <RelativeTime value={settings.secret_at} />
      </>,
    ],
    ['Events', 'Push'],
  ]
  if (settings.previous_until) {
    facts.push([
      'Previous secret',
      'Accepted until ' + time(settings.previous_until),
    ])
  }
  if (settings.rejection) {
    facts.push([
      'Last rejected',
      <>
        {settings.rejection.reason} ·{' '}
        {settings.rejection.provider || 'unknown provider'} ·{' '}
        <RelativeTime value={settings.rejection.at} />
      </>,
    ])
  }
  return (
    <>
      <Panel
        title="Push webhooks"
        detail="Signed GitHub and GitLab pushes start releases for enabled project branches."
        actions={
          <>
            <button className="button small" onClick={toggle}>
              <Power size={14} />
              {settings.enabled ? 'Stop deliveries' : 'Receive pushes'}
            </button>
            <button
              className="button small"
              onClick={() => setEditing('listener')}
            >
              <Pencil size={14} />
              Listener
            </button>
            {settings.tls && (
              <button
                className="button small"
                onClick={() => setEditing('certificate')}
              >
                <ShieldCheck size={14} />
                Certificate
              </button>
            )}
            <button className="button small" onClick={regenerate}>
              <KeyRound size={14} />
              Regenerate secret
            </button>
          </>
        }
      >
        <Facts items={facts} />
      </Panel>
      {editing === 'listener' && (
        <WebhookListener
          settings={settings}
          saved={setData}
          onClose={() => setEditing('')}
        />
      )}
      {editing === 'certificate' && (
        <WebhookCertificate
          settings={settings}
          saved={setData}
          onClose={() => setEditing('')}
        />
      )}
      {secret && (
        <Modal title="New webhook secret" onClose={() => setSecret('')}>
          <p className="modal-description">
            Copy it now and paste it into each repository webhook. It is not
            shown again.
          </p>
          <Code value={secret} copy />
          <div className="modal-actions">
            <button
              className="button primary"
              data-autofocus
              onClick={() => setSecret('')}
            >
              Done
            </button>
          </div>
        </Modal>
      )}
    </>
  )
}
