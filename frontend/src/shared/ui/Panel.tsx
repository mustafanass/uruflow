import { type ReactNode } from 'react'

export function Panel({
  title,
  detail,
  actions,
  children,
  className = '',
}: {
  title?: string
  detail?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={'panel ' + className}>
      {(title || actions) && (
        <div className="panel-heading">
          <div>
            {title && <h2>{title}</h2>}
            {detail && <p>{detail}</p>}
          </div>
          {actions}
        </div>
      )}
      {children}
    </section>
  )
}
