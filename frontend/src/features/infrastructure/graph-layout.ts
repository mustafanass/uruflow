interface Point {
  x: number
  y: number
}

export type Positions = Record<string, Point>

export function readPositions(
  key: string,
  size = { width: 276, height: 124 },
): Positions {
  try {
    const stored = JSON.parse(localStorage.getItem(key) || '{}')
    const envelope =
      stored?.version === 2 &&
      stored?.positions &&
      Number.isFinite(stored.width) &&
      Number.isFinite(stored.height) &&
      stored.width > 0 &&
      stored.height > 0
    const value: unknown = envelope ? stored.positions : stored
    const scale = {
      x: size.width / (envelope ? stored.width : 276),
      y: size.height / (envelope ? stored.height : 124),
    }
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return {}
    }
    return Object.fromEntries(
      Object.entries(value)
        .filter(
          ([, point]) =>
            point &&
            typeof point === 'object' &&
            Number.isFinite(point.x) &&
            Number.isFinite(point.y) &&
            Math.abs(point.x) < 1000000 &&
            Math.abs(point.y) < 1000000,
        )
        .map(([id, point]) => [
          id,
          { x: point.x * scale.x, y: point.y * scale.y },
        ]),
    )
  } catch {
    return {}
  }
}

export function equalPositions(a: Positions, b: Positions) {
  const keys = Object.keys(a)
  return (
    keys.length === Object.keys(b).length &&
    keys.every((key) => a[key].x === b[key]?.x && a[key].y === b[key]?.y)
  )
}

export function writePositions(
  key: string,
  positions: Positions,
  size: { width: number; height: number },
) {
  localStorage.setItem(key, JSON.stringify({ version: 2, ...size, positions }))
}
