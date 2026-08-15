/**
 * useLoopScroller — the horizontal magazine engine.
 *
 * A rAF-lerp track (no native scroll, so no snap fighting) driven by
 * wheel / pointer-drag / arrow keys. The panel list is rendered twice;
 * positions live on a modulo circle, so passing the end flows seamlessly
 * back into the beginning — infinite horizontal travel (the
 * canals-amsterdam / yourbana feel). Panels' [data-parallax] children
 * drift at their own speed and [data-reveal] children fade+rise as the
 * panel approaches center.
 */
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'

export interface LoopScroller {
  viewportRef: React.RefObject<HTMLDivElement | null>
  trackRef: React.RefObject<HTMLDivElement | null>
  /** index of the panel nearest viewport center */
  activeIndex: number
  scrollToPanel: (i: number) => void
  nudge: (dx: number) => void
}

const EASE = 0.085

export function useLoopScroller(panelCount: number): LoopScroller {
  const viewportRef = useRef<HTMLDivElement>(null)
  const trackRef = useRef<HTMLDivElement>(null)
  const target = useRef(0)
  const current = useRef(0)
  const offsets = useRef<number[]>([])
  const setWidth = useRef(1)
  const parallax = useRef<Array<{ el: HTMLElement; speed: number; panel: HTMLElement }>>([])
  const reveals = useRef<Array<{ el: HTMLElement; panel: HTMLElement }>>([])
  const [activeIndex, setActiveIndex] = useState(0)

  const measure = useCallback(() => {
    const track = trackRef.current
    if (!track || panelCount === 0) return
    // panels live inside two copy wrappers; measure the first copy
    const kids = Array.from(track.querySelectorAll<HTMLElement>('[data-panel]')).slice(0, panelCount)
    offsets.current = kids.map((k) => k.offsetLeft)
    setWidth.current = track.scrollWidth / 2 || 1
    parallax.current = []
    reveals.current = []
    for (const k of kids) {
      k.querySelectorAll<HTMLElement>('[data-parallax]').forEach((el) => {
        parallax.current.push({
          el,
          speed: parseFloat(el.dataset.parallax || '0.12'),
          panel: k,
        })
      })
      k.querySelectorAll<HTMLElement>('[data-reveal]').forEach((el) => {
        reveals.current.push({ el, panel: k })
      })
    }
  }, [panelCount])

  useLayoutEffect(() => {
    measure()
  }, [measure])

  useEffect(() => {
    const track = trackRef.current
    if (!track) return
    const ro = new ResizeObserver(() => measure())
    ro.observe(track)
    return () => ro.disconnect()
  }, [measure])

  /* ---------------- animation loop ---------------- */
  useEffect(() => {
    let raf = 0
    const loop = () => {
      const w = setWidth.current
      // shortest-path delta on the modulo circle
      let delta = target.current - current.current
      delta = ((delta % w) + w) % w
      if (delta > w / 2) delta -= w
      current.current += delta * EASE
      if (Math.abs(target.current - current.current) < 0.4) current.current = target.current
      // keep both coordinates inside one set (invisible, content duplicated)
      if (current.current >= w) {
        current.current -= w
        target.current -= w
      } else if (current.current < 0) {
        current.current += w
        target.current += w
      }
      if (trackRef.current) {
        trackRef.current.style.transform = `translate3d(${-current.current}px,0,0)`
      }

      const vw = viewportRef.current?.clientWidth ?? 1280
      const offs = offsets.current
      if (offs.length) {
        let idx = 0
        const x = ((current.current % w) + w) % w
        for (let i = 0; i < offs.length; i++) if (x + vw * 0.5 >= offs[i]) idx = i
        setActiveIndex((prev) => (prev === idx ? prev : idx))
      }
      for (const { el, speed, panel } of parallax.current) {
        const dx = panel.offsetLeft - current.current
        if (Math.abs(dx) < vw * 1.6) el.style.transform = `translate3d(${dx * speed}px,0,0)`
      }
      for (const { el, panel } of reveals.current) {
        const dx = panel.offsetLeft - current.current
        const p = Math.max(0, 1 - Math.abs(dx) / vw)
        el.style.opacity = String(0.15 + 0.85 * p)
        el.style.transform = `translateY(${(1 - p) * 26}px)`
      }
      raf = requestAnimationFrame(loop)
    }
    raf = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(raf)
  }, [])

  /* ---------------- inputs ---------------- */
  useEffect(() => {
    const vp = viewportRef.current
    if (!vp) return
    const onWheel = (e: WheelEvent) => {
      const d = Math.abs(e.deltaY) >= Math.abs(e.deltaX) ? e.deltaY : e.deltaX
      if (d !== 0) e.preventDefault()
      target.current += d * 1.15
    }
    vp.addEventListener('wheel', onWheel, { passive: false })
    return () => vp.removeEventListener('wheel', onWheel)
  }, [])

  useEffect(() => {
    const vp = viewportRef.current
    if (!vp) return
    let down = false
    let startX = 0
    let startTarget = 0
    let moved = 0
    const interactive = (t: EventTarget | null) =>
      t instanceof HTMLElement && !!t.closest('a,button,input,select,textarea,label,[role="button"]')
    const pd = (e: PointerEvent) => {
      if (e.pointerType === 'mouse' && interactive(e.target)) return
      down = true
      moved = 0
      startX = e.clientX
      startTarget = target.current
      vp.setPointerCapture(e.pointerId)
    }
    const pm = (e: PointerEvent) => {
      if (!down) return
      const dx = e.clientX - startX
      moved = Math.max(moved, Math.abs(dx))
      target.current = startTarget - dx * 1.5
    }
    const pu = () => {
      down = false
    }
    const clickCap = (e: MouseEvent) => {
      if (moved > 8) {
        e.preventDefault()
        e.stopPropagation()
      }
    }
    vp.addEventListener('pointerdown', pd)
    vp.addEventListener('pointermove', pm)
    vp.addEventListener('pointerup', pu)
    vp.addEventListener('pointercancel', pu)
    vp.addEventListener('click', clickCap, true)
    return () => {
      vp.removeEventListener('pointerdown', pd)
      vp.removeEventListener('pointermove', pm)
      vp.removeEventListener('pointerup', pu)
      vp.removeEventListener('pointercancel', pu)
      vp.removeEventListener('click', clickCap, true)
    }
  }, [])

  useEffect(() => {
    const kd = (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight') target.current += 520
      else if (e.key === 'ArrowLeft') target.current -= 520
    }
    window.addEventListener('keydown', kd)
    const vp = viewportRef.current
    vp?.addEventListener('keydown', kd)
    return () => {
      window.removeEventListener('keydown', kd)
      vp?.removeEventListener('keydown', kd)
    }
  }, [])

  const scrollToPanel = useCallback((i: number) => {
    const offs = offsets.current
    if (!offs.length) return
    const idx = ((i % offs.length) + offs.length) % offs.length
    target.current = offs[idx]
  }, [])

  const nudge = useCallback((dx: number) => {
    target.current += dx
  }, [])

  return { viewportRef, trackRef, activeIndex, scrollToPanel, nudge }
}
