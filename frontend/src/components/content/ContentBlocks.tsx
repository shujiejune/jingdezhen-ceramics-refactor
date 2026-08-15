/** Renders the backend's rich-content JSONB blocks (TDD §3.3). */
import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import type { ContentBlock } from '~/lib/types'

export function ContentBlocks({ blocks }: { blocks: ContentBlock[] }) {
  return (
    <div className="flex flex-col gap-6">
      {blocks.map((b, i) => {
        switch (b.type) {
          case 'heading':
            return (
              <h2 key={i} className="mt-6 text-[1.35rem] font-semibold tracking-tight text-ink-900">
                {b.text}
              </h2>
            )
          case 'paragraph':
            return (
              <p key={i} className="text-[1.02rem] leading-[1.9] text-ink-600">
                {b.text}
              </p>
            )
          case 'quote':
            return (
              <blockquote
                key={i}
                className="border-l-[3px] border-cobalt-300 py-1 pl-5 text-[1.05rem] leading-relaxed text-cobalt-800 italic"
              >
                {b.text}
              </blockquote>
            )
          case 'image':
            return (
              <figure key={i} className="mx-auto w-full max-w-md py-2">
                <div className="overflow-hidden rounded-xl border border-cobalt-100 bg-gradient-to-b from-wash to-porcelain">
                  <PorcelainFigure kind={b.figure_kind} seed={b.figure_seed} className="h-auto w-full" />
                </div>
                {b.caption && (
                  <figcaption className="mt-2.5 text-center text-[0.78rem] text-ink-400">{b.caption}</figcaption>
                )}
              </figure>
            )
          default:
            return null
        }
      })}
    </div>
  )
}
