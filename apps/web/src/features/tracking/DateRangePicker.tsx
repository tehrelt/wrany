interface Props {
  from: string
  to: string
  onFromChange: (v: string) => void
  onToChange: (v: string) => void
  onRefresh: () => void
}

export function DateRangePicker({ from, to, onFromChange, onToChange, onRefresh }: Props) {
  return (
    <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
      <label>
        From:&nbsp;
        <input type="datetime-local" value={from} onChange={(e) => onFromChange(e.target.value)} />
      </label>
      <label>
        To:&nbsp;
        <input type="datetime-local" value={to} onChange={(e) => onToChange(e.target.value)} />
      </label>
      <button onClick={onRefresh}>Refresh</button>
    </div>
  )
}
