import { formatAxisTime, formatRate } from '../lib/format';

interface SparklineProps {
  points: Array<[string, number | null]>;
  label?: string;
  height?: number;
  stroke?: string;
  formatValue?: (n: number) => string;
}

const AXIS_LEFT = 44;
const AXIS_BOTTOM = 22;
const PLOT_RIGHT = 6;
const PLOT_TOP = 4;

type NumericPoint = [string, number];

function numericPoints(points: Array<[string, number | null]>): NumericPoint[] {
  const out: NumericPoint[] = [];
  for (const [ts, v] of points) {
    if (v != null) out.push([ts, v]);
  }
  return out;
}

function plotMetrics(values: number[]) {
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const mid = min + span / 2;
  return { min, max, mid, span };
}

function pathFromPoints(
  numeric: NumericPoint[],
  plotWidth: number,
  plotHeight: number,
  min: number,
  span: number,
): string {
  if (numeric.length === 0) return '';
  const step = numeric.length > 1 ? plotWidth / (numeric.length - 1) : 0;

  return numeric
    .map(([, value], i) => {
      const x = i * step;
      const y = plotHeight - ((value - min) / span) * (plotHeight - PLOT_TOP * 2) - PLOT_TOP;
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
}

function yAt(value: number, plotHeight: number, min: number, span: number): number {
  return plotHeight - ((value - min) / span) * (plotHeight - PLOT_TOP * 2) - PLOT_TOP;
}

export function Sparkline({
  points,
  label,
  height = 88,
  stroke = 'var(--accent)',
  formatValue = formatRate,
}: Readonly<SparklineProps>) {
  if (points.length === 0) {
    return <div class="sparkline empty">No series data</div>;
  }

  const numeric = numericPoints(points);
  if (numeric.length === 0) {
    return <div class="sparkline empty">No series data</div>;
  }

  const values = numeric.map(([, v]) => v);
  const { min, max, mid, span } = plotMetrics(values);
  const plotWidth = 272;
  const plotHeight = height - AXIS_BOTTOM;
  const width = AXIS_LEFT + plotWidth + PLOT_RIGHT;
  const d = pathFromPoints(numeric, plotWidth, plotHeight, min, span);
  const last = numeric[numeric.length - 1]?.[1];
  const firstTs = numeric[0]?.[0];
  const lastTs = numeric[numeric.length - 1]?.[0];
  const yTicks = min === max ? [min] : [max, mid, min];
  const gridColor = 'var(--border-soft)';
  const axisColor = 'var(--text-dim)';

  return (
    <div class="sparkline">
      {label && (
        <div class="sparkline-label">
          <span>{label}</span>
          {last != null && <span class="sparkline-latest">{formatValue(last)}</span>}
        </div>
      )}
      <svg viewBox={`0 0 ${width} ${height}`} width="100%" height={height} role="img" aria-label={label ? `${label} time series` : 'Time series'}>
        <g transform={`translate(${AXIS_LEFT}, 0)`}>
          {yTicks.map((tick) => {
            const y = yAt(tick, plotHeight, min, span);
            return (
              <g key={tick}>
                <line x1={0} y1={y} x2={plotWidth} y2={y} stroke={gridColor} stroke-width="1" stroke-dasharray="3 4" />
              </g>
            );
          })}
          <path d={d} fill="none" stroke={stroke} stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          <path d={`${d} L${plotWidth},${plotHeight} L0,${plotHeight} Z`} fill={stroke} opacity="0.1" stroke="none" />
          <line x1={0} y1={plotHeight} x2={plotWidth} y2={plotHeight} stroke={gridColor} stroke-width="1" />
        </g>
        {yTicks.map((tick) => {
          const y = yAt(tick, plotHeight, min, span);
          return (
            <text
              key={`y-${tick}`}
              x={AXIS_LEFT - 6}
              y={y + 3}
              text-anchor="end"
              fill={axisColor}
              font-size="9"
              font-family="var(--mono)"
            >
              {formatValue(tick)}
            </text>
          );
        })}
        <text x={AXIS_LEFT} y={height - 4} text-anchor="start" fill={axisColor} font-size="9" font-family="var(--mono)">
          {formatAxisTime(firstTs)}
        </text>
        <text x={AXIS_LEFT + plotWidth} y={height - 4} text-anchor="end" fill={axisColor} font-size="9" font-family="var(--mono)">
          {formatAxisTime(lastTs)}
        </text>
      </svg>
      <div class="sparkline-meta mono">
        <span>min {formatValue(min)} · max {formatValue(max)}</span>
        <span>{formatAxisTime(firstTs)} – {formatAxisTime(lastTs)}</span>
      </div>
    </div>
  );
}
