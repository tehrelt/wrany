export function EmptyState({ message = 'No data found for this time range.' }: { message?: string }) {
  return <div style={{ padding: 16, color: '#888' }}>{message}</div>
}
