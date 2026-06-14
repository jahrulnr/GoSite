import type { SeriesPoint } from '../api/types';

interface SparklineProps {
  points: SeriesPoint[];
  label?: string;
  height?: number;
  stroke?: string;
}

function pathFromPoints(points: SeriesPoint[], width: number, height: number): string {
  if (points.length === 0) return '';
  const values = points.map(([, v]) => v);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const step = points.length > 1 ? width / (points.length - 1) : 0;

  return points
    .map(([_, value], i) => {
      const x = i * step;
      const y = height - ((value - min) / span) * (height - 4) - 2;
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
}

export function Sparkline({ points, label, height = 72, stroke = 'var(--accent)' }: Readonly<SparklineProps>) {
  if (points.length === 0) {
    return <div class="sparkline empty">No series data</div>;
  }
  const width = 320;
  const d = pathFromPoints(points, width, height);
  const last = points[points.length - 1]?.[1] ?? 0;

  return (
    <div class="sparkline">
      {label && <div class="sparkline-label">{label}</div>}
      <svg viewBox={`0 0 ${width} ${height}`} width="100%" height={height} aria-hidden="true">
        <path d={d} fill="none" stroke={stroke} stroke-width="2" stroke-linecap="round" />
        <path d={`${d} L${width},${height} L0,${height} Z`} fill={stroke} opacity="0.12" stroke="none" />
      </svg>
      <div class="sparkline-meta mono">{last.toLocaleString()} latest</div>
    </div>
  );
}
