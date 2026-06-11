export function ErrorState({ message = 'Something went wrong.' }: { message?: string }) {
  return <div style={{ padding: 16, color: '#c00' }}>Error: {message}</div>
}
