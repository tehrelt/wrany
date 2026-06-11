import { useEffect } from 'react'
import { MapContainer, TileLayer, Polyline, Marker, useMap } from 'react-leaflet'
import { LatLngTuple } from 'leaflet'
import { TrackingPoint } from '../features/tracking/trackingApi'

interface FitBoundsProps {
  positions: LatLngTuple[]
}

function FitBounds({ positions }: FitBoundsProps) {
  const map = useMap()
  useEffect(() => {
    if (positions.length > 0) {
      map.fitBounds(positions, { padding: [32, 32] })
    }
  }, [map, positions])
  return null
}

interface Props {
  points: TrackingPoint[]
}

export function MapView({ points }: Props) {
  const positions: LatLngTuple[] = points.map((p) => [p.lat, p.lon])
  const defaultCenter: LatLngTuple = [55.751244, 37.618423]
  const defaultZoom = 12

  return (
    <MapContainer
      center={defaultCenter}
      zoom={defaultZoom}
      style={{ height: 400, width: '100%', borderRadius: 6 }}
    >
      <TileLayer
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
      />
      {positions.length > 0 && (
        <>
          <Polyline positions={positions} color="#2563eb" weight={2} />
          <Marker position={positions[0]} />
          {positions.length > 1 && <Marker position={positions[positions.length - 1]} />}
          <FitBounds positions={positions} />
        </>
      )}
    </MapContainer>
  )
}
