"""Runs one download on the stand and records it: a screenshot every 2 s, a video, and the
progress wave's state every 0.1 s (to check that it never stops, jumps or goes back).

    pip install playwright && playwright install chromium
    python stand/record.py audio-list 55 out-folder     # audio | audio-list | video | video-list
"""
import json, os, shutil, subprocess, sys, time
from playwright.sync_api import sync_playwright

here = os.path.dirname(os.path.abspath(__file__))
mode = sys.argv[1] if len(sys.argv) > 1 else 'audio-list'
secs = int(sys.argv[2]) if len(sys.argv) > 2 else 60
out = sys.argv[3] if len(sys.argv) > 3 else '/tmp/tyaga-stand/run'
url = subprocess.check_output([os.path.join(here, 'run.sh')]).decode().strip()
shutil.rmtree(out, ignore_errors=True); os.makedirs(out)
with sync_playwright() as p:
    b = p.chromium.launch(executable_path=os.environ.get('CHROME') or None)
    ctx = b.new_context(viewport={'width': 1100, 'height': 900}, color_scheme='dark',
                        record_video_dir=out, record_video_size={'width': 1100, 'height': 900})
    pg = ctx.new_page()
    errs = []; pg.on('pageerror', lambda e: errs.append(str(e)))
    pg.goto(url); pg.wait_for_function("document.querySelector('#setup').hidden", timeout=30000)
    pg.evaluate('localStorage.clear()')
    if mode.startswith('audio'):
        pg.click('#mode button[data-v=audio]'); pg.click('#format button[data-v=mp3]')
    link = 'https://www.youtube.com/watch?v=Ab3dEf5Gh7J'
    if mode.endswith('list'):
        link += '&list=PLxyz'; pg.check('#playlist')
        if mode.startswith('audio'): pg.check('#merge')
    pg.fill('#links', link); pg.click('#go')
    pg.evaluate('window.__s=[];setInterval(()=>{for(const id in waveState){const s=waveState[id];'
                'const c=document.querySelector(`canvas.wave[data-id="${id}"]`);if(c&&c.offsetParent)'
                'window.__s.push([performance.now(),+id,s.phase,s.boost,s.shown])}},100)')
    t0, k = time.time(), 0
    while time.time() - t0 < secs:
        pg.screenshot(path=f'{out}/s{k:03d}.png'); k += 1
        time.sleep(2)
    samples = pg.evaluate('window.__s')
    json.dump(samples, open(f'{out}/wave.json', 'w'))
    ctx.close(); b.close()

# The wave: phase must only go forward, never stop while downloading; how fast it ran.
v = samples  # only while the wave is on screen
rates = [(v[i+1][2]-v[i][2]) / ((v[i+1][0]-v[i][0]) / 1000) for i in range(len(v)-1) if v[i+1][0] > v[i][0]]
print('page errors:', errs or 'none')
if rates:
    print(f'wave while downloading: speed {min(rates):.1f}…{max(rates):.1f} rad/s, '
          f'went back {sum(1 for i in range(len(v)-1) if v[i+1][2] < v[i][2])} times')
print('screenshots and video in', out)
