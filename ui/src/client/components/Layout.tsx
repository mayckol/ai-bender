import type { ComponentChildren } from 'preact';

interface Props {
  title: string;
  breadcrumb?: string;
  right?: ComponentChildren;
  children: ComponentChildren;
}

export function Layout({ title, breadcrumb, right, children }: Props) {
  return (
    <div class="shell">
      <header class="header">
        <div>
          <h1>{title}</h1>
          {breadcrumb && <div class="breadcrumb">{breadcrumb}</div>}
        </div>
        <div>{right}</div>
      </header>
      {children}
    </div>
  );
}
