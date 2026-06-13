export const mapTokens = {
  routePrimary: '#2F7DFF',
  routeGlow: 'rgba(47, 125, 255, 0.25)',
  routeInactive: 'rgba(47, 125, 255, 0.34)',
  start: '#16A34A',
  finish: '#EF4444',
  selected: '#D97706',
  background: '#F4F5F7',
  mutedText: '#6B7280',
}

export const yandexTelemetryMapStyle = {
  style: [
    {
      tags: { any: ['landscape', 'admin', 'land', 'transit'] },
      elements: 'geometry',
      stylers: [{ color: mapTokens.background }],
    },
    {
      tags: { any: ['building'] },
      elements: 'geometry',
      stylers: [{ color: '#E7E9ED' }, { opacity: 0.58 }],
    },
    {
      tags: { any: ['park', 'vegetation'] },
      elements: 'geometry',
      stylers: [{ color: '#DCE5DA' }, { opacity: 0.72 }],
    },
    {
      tags: { any: ['water'] },
      elements: 'geometry',
      stylers: [{ color: '#D3DEE7' }],
    },
    {
      tags: { any: ['road'] },
      elements: 'geometry',
      stylers: [{ color: '#FFFFFF' }, { opacity: 0.88 }],
    },
    {
      tags: { any: ['poi'] },
      elements: 'icon',
      stylers: [{ visibility: 'off' }],
    },
    {
      tags: { any: ['poi'] },
      elements: 'label',
      stylers: [{ color: '#9CA3AF' }, { opacity: 0.18 }],
    },
    {
      tags: { any: ['transit', 'transit_location'] },
      elements: 'icon',
      stylers: [{ visibility: 'off' }],
    },
    {
      tags: { any: ['transit', 'transit_location'] },
      elements: 'label',
      stylers: [{ visibility: 'off' }],
    },
    {
      tags: { any: ['locality', 'address'] },
      elements: 'label',
      stylers: [{ color: mapTokens.mutedText }, { opacity: 0.72 }],
    },
  ],
}
