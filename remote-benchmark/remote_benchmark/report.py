import argparse
import html
import json
import re
from datetime import datetime, timezone
from pathlib import Path

import yaml

BLOCK_RE = re.compile(r"^block (?P<height>\d+) txs=(?P<txs>\d+)(?P<rest>.*)$")
METRIC_RE = re.compile(r"^(?P<name>[a-z][a-z0-9_]*) (?P<value>.+)$")
TIMESTAMP_RE = re.compile(r"\b(?P<timestamp>\d{4}-\d{2}-\d{2}[T ]\S+)")


def parse_stats(text: str) -> tuple[list[dict], dict[str, str]]:
    blocks = []
    metrics = {}
    for line in text.splitlines():
        block_match = BLOCK_RE.match(line)
        if block_match:
            rest = block_match.group("rest")
            tps_match = re.search(r"\btps=([0-9.]+)", rest)
            gas_match = re.search(r"\bgas=(\d+)", rest)
            timestamp_match = TIMESTAMP_RE.search(rest)
            blocks.append(
                {
                    "height": int(block_match.group("height")),
                    "transactions": int(block_match.group("txs")),
                    "gas_consumed": int(gas_match.group(1)) if gas_match else 0,
                    "tps": float(tps_match.group(1)) if tps_match else 0,
                    "timestamp": (
                        timestamp_match.group("timestamp") if timestamp_match else None
                    ),
                }
            )
            continue

        metric_match = METRIC_RE.match(line)
        if metric_match:
            metrics[metric_match.group("name")] = metric_match.group("value")
    return blocks, metrics


def bucket_by_second(blocks: list[dict]) -> list[dict]:
    """Aggregate the active transaction window into wall-clock seconds."""
    timestamped = []
    for block in blocks:
        if not block["timestamp"]:
            continue
        timestamped.append((datetime.fromisoformat(block["timestamp"]), block))

    active = [item for item in timestamped if item[1]["transactions"] > 0]
    if not active:
        return []

    first_second = int(active[0][0].timestamp())
    last_second = int(active[-1][0].timestamp())
    totals = {
        second: {"transactions": 0, "gas_consumed": 0}
        for second in range(first_second, last_second + 1)
    }
    for timestamp, block in timestamped:
        second = int(timestamp.timestamp())
        if second in totals:
            totals[second]["transactions"] += block["transactions"]
            totals[second]["gas_consumed"] += block["gas_consumed"]

    result = []
    transaction_counts = []
    for second, values in totals.items():
        transaction_counts.append(values["transactions"])
        rolling_window = transaction_counts[-5:]
        result.append(
            {
                "elapsed_second": second - first_second,
                "timestamp": datetime.fromtimestamp(second, timezone.utc).isoformat(),
                **values,
                "rolling_tps_5s": sum(rolling_window) / len(rolling_window),
            }
        )
    return result


def _flatten(value, prefix=""):
    if isinstance(value, dict):
        for key, child in value.items():
            name = f"{prefix}.{key}" if prefix else str(key)
            yield from _flatten(child, name)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from _flatten(child, f"{prefix}[{index}]")
    else:
        yield prefix, value


def _display(value) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):
        return str(value).lower()
    return str(value)


def render_report(
    config: dict,
    stats_text: str,
    generated_at: datetime,
    validators: int | None = None,
    testcase: str | None = None,
    start_account: int | None = None,
    end_account: int | None = None,
) -> str:
    blocks, metrics = parse_stats(stats_text)
    second_buckets = bucket_by_second(blocks)
    params = {
        "benchmark.validators": validators,
        "benchmark.testcase": testcase,
        "benchmark.start_account": start_account,
        "benchmark.end_account": end_account,
        "benchmark.generated_at": generated_at.astimezone().isoformat(
            timespec="seconds"
        ),
    }
    params.update(dict(_flatten(config)))
    param_rows = "\n".join(
        f"<tr><th>{html.escape(name)}</th><td>{html.escape(_display(value))}</td></tr>"
        for name, value in params.items()
        if value is not None
    )

    featured = [
        ("Peak TPS", metrics.get("peak_tps", "N/A")),
        ("Overall TPS", metrics.get("overall_tps", "N/A")),
        ("Total transactions", metrics.get("total_txs", "N/A")),
        ("Committed Cosmos txs", metrics.get("committed_cosmos_txs", "N/A")),
    ]
    if second_buckets:
        featured.extend(
            [
                (
                    "Peak 1s TPS",
                    f"{max(bucket['transactions'] for bucket in second_buckets):,}",
                ),
                (
                    "Peak 5s avg TPS",
                    f"{max(bucket['rolling_tps_5s'] for bucket in second_buckets):,.1f}",
                ),
                (
                    "Peak gas / second",
                    f"{max(bucket['gas_consumed'] for bucket in second_buckets):,}",
                ),
            ]
        )
    metric_cards = "\n".join(
        f'<div class="metric"><span>{html.escape(label)}</span>'
        f"<strong>{html.escape(value)}</strong></div>"
        for label, value in featured
    )
    chart_data = json.dumps(blocks, separators=(",", ":")).replace("<", "\\u003c")
    second_chart_data = json.dumps(second_buckets, separators=(",", ":")).replace(
        "<", "\\u003c"
    )
    title_bits = [str(validators) + " validator" + ("s" if validators != 1 else "")]
    if testcase:
        title_bits.append(testcase)
    title = " / ".join(title_bits) if validators else "Benchmark"
    generated_label = generated_at.astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")

    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{html.escape(title)} benchmark report</title>
  <style>
    :root {{ color-scheme: light; --ink:#182026; --muted:#66717a; --line:#d8dee3;
      --surface:#fff; --page:#f3f5f6; --accent:#087e8b; --bar:#ff5a5f; }}
    * {{ box-sizing:border-box; }}
    body {{ margin:0; background:var(--page); color:var(--ink); font:14px/1.5 system-ui,sans-serif; }}
    main {{ width:min(1180px, calc(100% - 32px)); margin:0 auto; padding:32px 0 56px; }}
    h1 {{ margin:0; font-size:28px; letter-spacing:0; }}
    h2 {{ margin:32px 0 12px; font-size:18px; letter-spacing:0; }}
    .timestamp {{ margin:4px 0 24px; color:var(--muted); }}
    .params {{ overflow:auto; border:1px solid var(--line); background:var(--surface); }}
    table {{ width:100%; border-collapse:collapse; }}
    th,td {{ padding:9px 12px; border-bottom:1px solid var(--line); text-align:left; vertical-align:top; }}
    th {{ width:32%; color:#344047; font-weight:600; background:#fafbfb; }}
    tr:last-child th,tr:last-child td {{ border-bottom:0; }}
    td {{ overflow-wrap:anywhere; }}
    .metrics {{ display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:12px; }}
    .metric {{ min-width:0; padding:14px 16px; border:1px solid var(--line); background:var(--surface); }}
    .metric span {{ display:block; color:var(--muted); font-size:12px; }}
    .metric strong {{ display:block; margin-top:3px; font-size:21px; font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }}
    .chart-wrap {{ position:relative; height:430px; border:1px solid var(--line); background:var(--surface); padding:12px; }}
    canvas {{ width:100%; height:100%; display:block; }}
    .tooltip {{ position:absolute; display:none; pointer-events:none; padding:7px 9px; color:#fff;
      background:#182026; font-size:12px; border-radius:4px; white-space:nowrap; }}
    .empty {{ height:100%; display:grid; place-items:center; color:var(--muted); }}
    @media (max-width:760px) {{
      main {{ width:min(100% - 20px, 1180px); padding-top:20px; }}
      h1 {{ font-size:23px; }} .metrics {{ grid-template-columns:repeat(2,minmax(0,1fr)); }}
      .chart-wrap {{ height:340px; }} th {{ width:45%; }}
    }}
  </style>
</head>
<body>
<main>
  <h1>{html.escape(title)} benchmark report</h1>
  <p class="timestamp">Generated {html.escape(generated_label)}</p>
  <h2>Parameters</h2>
  <div class="params"><table><tbody>{param_rows}</tbody></table></div>
  <h2>Results</h2>
  <div class="metrics">{metric_cards}</div>
  <h2>Transactions by block</h2>
  <div class="chart-wrap" id="chartWrap">
    <canvas id="chart" role="img" aria-label="Transaction count for each block height"></canvas>
    <div class="tooltip" id="tooltip"></div>
  </div>
  <h2>Gas consumed by block</h2>
  <div class="chart-wrap" id="gasChartWrap">
    <canvas id="gasChart" role="img" aria-label="Gas consumed for each block height"></canvas>
    <div class="tooltip" id="gasTooltip"></div>
  </div>
  <h2>Transactions per second</h2>
  <div class="chart-wrap" id="secondChartWrap">
    <canvas id="secondChart" role="img" aria-label="Transactions committed per elapsed second"></canvas>
    <div class="tooltip" id="secondTooltip"></div>
  </div>
  <h2>Gas consumed per second</h2>
  <div class="chart-wrap" id="secondGasChartWrap">
    <canvas id="secondGasChart" role="img" aria-label="Gas consumed per elapsed second"></canvas>
    <div class="tooltip" id="secondGasTooltip"></div>
  </div>
</main>
<script>
const data={chart_data};
const secondData={second_chart_data};
function createBarChart(canvasId,tooltipId,valueKey,yLabel,color,tooltipText) {{
  const canvas=document.getElementById(canvasId), wrap=canvas.parentElement;
  const tip=document.getElementById(tooltipId), ctx=canvas.getContext('2d');
  let bars=[];
  function draw() {{
    const dpr=window.devicePixelRatio||1, rect=canvas.getBoundingClientRect();
    canvas.width=Math.max(1,Math.round(rect.width*dpr)); canvas.height=Math.max(1,Math.round(rect.height*dpr));
    ctx.setTransform(dpr,0,0,dpr,0,0); ctx.clearRect(0,0,rect.width,rect.height); bars=[];
    if(!data.length) {{ ctx.fillStyle='#66717a'; ctx.textAlign='center'; ctx.fillText('No block data recorded',rect.width/2,rect.height/2); return; }}
    const max=Math.max(1,...data.map(d=>d[valueKey]));
    ctx.strokeStyle='#d8dee3'; ctx.fillStyle='#66717a'; ctx.font='12px system-ui'; ctx.lineWidth=1;
    const tickLabels=Array.from({{length:5}},(_,i)=>Math.round(max*i/4).toLocaleString());
    const maxTickWidth=Math.max(...tickLabels.map(label=>ctx.measureText(label).width));
    const pad={{l:Math.max(76,Math.ceil(maxTickWidth)+36),r:20,t:20,b:48}};
    const w=rect.width-pad.l-pad.r, h=rect.height-pad.t-pad.b;
    tickLabels.forEach((label,i)=>{{ const y=pad.t+h-h*i/4; ctx.beginPath(); ctx.moveTo(pad.l,y); ctx.lineTo(pad.l+w,y); ctx.stroke(); ctx.textAlign='right'; ctx.fillText(label,pad.l-9,y+4); }});
    const slot=w/data.length, bw=Math.max(1,Math.min(28,slot*.72));
    data.forEach((d,i)=>{{ const bh=h*d[valueKey]/max, x=pad.l+slot*i+(slot-bw)/2, y=pad.t+h-bh;
      ctx.fillStyle=color; ctx.fillRect(x,y,bw,bh); bars.push({{x,y,w:bw,h:bh,d}}); }});
    const ticks=Math.min(8,data.length); ctx.fillStyle='#66717a'; ctx.textAlign='center';
    for(let i=0;i<ticks;i++) {{ const idx=ticks===1?0:Math.round(i*(data.length-1)/(ticks-1)); ctx.fillText(data[idx].height,pad.l+slot*idx+slot/2,pad.t+h+20); }}
    ctx.save(); ctx.translate(16,pad.t+h/2); ctx.rotate(-Math.PI/2); ctx.fillText(yLabel,0,0); ctx.restore();
    ctx.fillText('Block height',pad.l+w/2,rect.height-8);
  }}
  canvas.addEventListener('mousemove',e=>{{ const r=canvas.getBoundingClientRect(), x=e.clientX-r.left, y=e.clientY-r.top;
    const hit=bars.find(b=>x>=b.x&&x<=b.x+b.w&&y>=Math.min(b.y,b.y+b.h)&&y<=b.y+b.h);
    if(!hit) {{ tip.style.display='none'; return; }}
    tip.textContent=tooltipText(hit.d); tip.style.display='block';
    tip.style.left=Math.min(x+12,wrap.clientWidth-tip.offsetWidth-8)+'px'; tip.style.top=Math.max(8,y-tip.offsetHeight-8)+'px';
  }});
  canvas.addEventListener('mouseleave',()=>tip.style.display='none');
  new ResizeObserver(draw).observe(wrap); draw();
}}
createBarChart('chart','tooltip','transactions','Transactions','#ff5a5f',d=>
  `Block ${{d.height}}: ${{d.transactions.toLocaleString()}} txs, ${{d.tps.toLocaleString()}} TPS`);
createBarChart('gasChart','gasTooltip','gas_consumed','Gas consumed','#087e8b',d=>
  `Block ${{d.height}}: ${{d.gas_consumed.toLocaleString()}} gas consumed`);
function createSecondChart(canvasId,tooltipId,valueKey,yLabel,color,rolling=false) {{
  const canvas=document.getElementById(canvasId), wrap=canvas.parentElement;
  const tip=document.getElementById(tooltipId), ctx=canvas.getContext('2d');
  let points=[];
  function draw() {{
    const dpr=window.devicePixelRatio||1, rect=canvas.getBoundingClientRect();
    canvas.width=Math.max(1,Math.round(rect.width*dpr)); canvas.height=Math.max(1,Math.round(rect.height*dpr));
    ctx.setTransform(dpr,0,0,dpr,0,0); ctx.clearRect(0,0,rect.width,rect.height); points=[];
    if(!secondData.length) {{ ctx.fillStyle='#66717a'; ctx.textAlign='center'; ctx.fillText('No timestamped transaction data recorded',rect.width/2,rect.height/2); return; }}
    const values=secondData.flatMap(d=>rolling?[d[valueKey],d.rolling_tps_5s]:[d[valueKey]]);
    const max=Math.max(1,...values), tickLabels=Array.from({{length:5}},(_,i)=>Math.round(max*i/4).toLocaleString());
    ctx.font='12px system-ui'; const maxTickWidth=Math.max(...tickLabels.map(label=>ctx.measureText(label).width));
    const pad={{l:Math.max(76,Math.ceil(maxTickWidth)+36),r:20,t:20,b:48}}, w=rect.width-pad.l-pad.r, h=rect.height-pad.t-pad.b;
    ctx.strokeStyle='#d8dee3'; ctx.fillStyle='#66717a'; ctx.lineWidth=1;
    tickLabels.forEach((label,i)=>{{ const y=pad.t+h-h*i/4; ctx.beginPath(); ctx.moveTo(pad.l,y); ctx.lineTo(pad.l+w,y); ctx.stroke(); ctx.textAlign='right'; ctx.fillText(label,pad.l-9,y+4); }});
    const slot=w/secondData.length, bw=Math.max(1,Math.min(32,slot*.72));
    secondData.forEach((d,i)=>{{ const bh=h*d[valueKey]/max, x=pad.l+slot*i+(slot-bw)/2, y=pad.t+h-bh;
      ctx.fillStyle=color; ctx.fillRect(x,y,bw,bh); points.push({{x:pad.l+slot*i+slot/2,y,d}}); }});
    if(rolling) {{ ctx.beginPath(); ctx.strokeStyle='#182026'; ctx.lineWidth=2;
      secondData.forEach((d,i)=>{{ const x=pad.l+slot*i+slot/2, y=pad.t+h-h*d.rolling_tps_5s/max; i?ctx.lineTo(x,y):ctx.moveTo(x,y); }}); ctx.stroke();
      ctx.fillStyle='#182026'; ctx.fillRect(pad.l+8,pad.t+4,18,2); ctx.fillText('5-second moving average',pad.l+32,pad.t+9);
    }}
    const ticks=Math.min(8,secondData.length); ctx.fillStyle='#66717a'; ctx.textAlign='center';
    for(let i=0;i<ticks;i++) {{ const idx=ticks===1?0:Math.round(i*(secondData.length-1)/(ticks-1)); ctx.fillText(secondData[idx].elapsed_second+'s',pad.l+slot*idx+slot/2,pad.t+h+20); }}
    ctx.save(); ctx.translate(16,pad.t+h/2); ctx.rotate(-Math.PI/2); ctx.fillText(yLabel,0,0); ctx.restore();
    ctx.fillText('Elapsed time from first committed transaction',pad.l+w/2,rect.height-8);
  }}
  canvas.addEventListener('mousemove',e=>{{ const r=canvas.getBoundingClientRect(), x=e.clientX-r.left;
    const hit=points.reduce((best,p)=>!best||Math.abs(p.x-x)<Math.abs(best.x-x)?p:best,null); if(!hit) return;
    const d=hit.d, when=new Date(d.timestamp).toLocaleTimeString();
    tip.textContent=valueKey==='transactions'
      ? `${{d.elapsed_second}}s (${{when}}): ${{d.transactions.toLocaleString()}} TPS; 5s avg ${{d.rolling_tps_5s.toLocaleString(undefined,{{maximumFractionDigits:1}})}} TPS`
      : `${{d.elapsed_second}}s (${{when}}): ${{d.gas_consumed.toLocaleString()}} gas`;
    tip.style.display='block'; tip.style.left=Math.min(x+12,wrap.clientWidth-tip.offsetWidth-8)+'px'; tip.style.top='8px';
  }});
  canvas.addEventListener('mouseleave',()=>tip.style.display='none');
  new ResizeObserver(draw).observe(wrap); draw();
}}
createSecondChart('secondChart','secondTooltip','transactions','Transactions / second','#ff5a5f',true);
createSecondChart('secondGasChart','secondGasTooltip','gas_consumed','Gas / second','#087e8b');
</script>
</body>
</html>
"""


def generate_report(
    config_path: Path,
    stats_path: Path,
    output_path: Path,
    generated_at: datetime,
    validators: int | None = None,
    testcase: str | None = None,
    start_account: int | None = None,
    end_account: int | None = None,
) -> None:
    config = yaml.safe_load(config_path.read_text())
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        render_report(
            config,
            stats_path.read_text(),
            generated_at,
            validators=validators,
            testcase=testcase,
            start_account=start_account,
            end_account=end_account,
        )
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate a benchmark HTML report")
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--stats", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--timestamp", required=True)
    parser.add_argument("--validators", type=int)
    parser.add_argument("--testcase")
    parser.add_argument("--start-account", type=int)
    parser.add_argument("--end-account", type=int)
    args = parser.parse_args()
    generated_at = datetime.fromisoformat(args.timestamp)
    generate_report(
        args.config,
        args.stats,
        args.output,
        generated_at,
        validators=args.validators,
        testcase=args.testcase,
        start_account=args.start_account,
        end_account=args.end_account,
    )


if __name__ == "__main__":
    main()
