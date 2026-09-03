/**
 * Mock dataset (PROTOTYPE) — bilingual content records shaped like the
 * backend's tables (models/*.go). Transport-layer reads resolve the
 * requested locale's "translation row" exactly like the Go services do.
 *
 * Dates are fixed ISO strings so SSR and client renders match.
 * Money is minor units (fen) throughout, authored in CNY (TDD §7).
 */
import type { ContentBlock, SKUAttributes } from '~/lib/types'

export interface Translation<T> {
  enUS: T
  zhCN: T
}

export interface ProductRecord {
  id: number
  artistId: number
  category: string
  tags: string[] // canonical keys
  figureSeed: number
  figureKind: 'vase' | 'bowl' | 'plate' | 'teapot' | 'jar'
  featured?: boolean
  createdAt: string
  publishedAt: string
  translations: Translation<{
    title: string
    slug: string
    description: string
    metaTitle: string
    metaDescription: string
  }>
  skus: Array<{
    id: number
    skuCode: string
    priceCny: number
    stock: number
    weightGrams: number
    lowStockThreshold: number
    attributes: Translation<SKUAttributes>
  }>
}

export interface ArtistRecord {
  id: number
  glyph: string
  createdAt: string
  translations: Translation<{
    name: string
    slug: string
    bio: string
  }>
}

export interface StoryRecord {
  id: number
  startYear: number
  figureSeed: number
  translations: Translation<{
    title: string
    slug: string
    summary: string
    content: ContentBlock[]
  }>
}

export interface ActivityRecord {
  id: number
  type: 'destination' | 'lifestyle'
  figureSeed: number
  lat?: number
  lng?: number
  address?: Translation<string>
  opening?: Translation<string>
  translations: Translation<{
    title: string
    slug: string
    summary: string
    content: ContentBlock[]
  }>
}

/* ------------------------------------------------------------------ */
/* Artists                                                             */
/* ------------------------------------------------------------------ */

export const ARTISTS: ArtistRecord[] = [
  {
    id: 1,
    glyph: '陈',
    createdAt: '2025-03-02T09:00:00Z',
    translations: {
      enUS: {
        name: 'Chen Yuqing',
        slug: 'chen-yuqing',
        bio: 'Third-generation qinghua brush painter Chen Yuqing learned the “one-stroke dragon” from her grandmother in the old kiln district. Her landscape vases carry the river mist of the Chang into contemporary collections across Europe and North America. She paints every piece freehand; no two are identical.',
      },
      zhCN: {
        name: '陈雨晴',
        slug: 'chen-yuqing',
        bio: '青花第三代画师陈雨晴，幼年随祖母在老窑区习得“一笔龙”技法。她的山水瓶将昌江烟雨带入欧美当代收藏界。每件作品皆徒手绘制，无一雷同。',
      },
    },
  },
  {
    id: 2,
    glyph: '林',
    createdAt: '2025-03-04T09:00:00Z',
    translations: {
      enUS: {
        name: 'Lin Haiming',
        slug: 'lin-haiming',
        bio: 'Sculptor Lin Haiming fires his celadon forms for seventy-two hours in a wood kiln at the edge of Sanbao valley, letting flame and ash finish each surface. His “River Mist” series treats porcelain as frozen weather — a moment of fog held at 1,300°C.',
      },
      zhCN: {
        name: '林海鸣',
        slug: 'lin-haiming',
        bio: '雕塑家林海鸣在三宝谷深处的柴窑中烧制青瓷七十二小时，让火焰与落灰为每件作品收尾。他的“江雾”系列将瓷视为凝固的天气——一千三百度锁住的一团雾。',
      },
    },
  },
  {
    id: 3,
    glyph: '苏',
    createdAt: '2025-03-05T09:00:00Z',
    translations: {
      enUS: {
        name: 'Su Wanqing',
        slug: 'su-wanqing',
        bio: 'Famille-rose enamel artist Su Wanqing restores imperial porcelain by day and paints her own quiet gardens by night. Her paired cups revive the “chicken cup” palette with a modern restraint that collectors call “a breath held in color.”',
      },
      zhCN: {
        name: '苏婉清',
        slug: 'su-wanqing',
        bio: '粉彩艺术家苏婉清白天修复官窑瓷器，夜晚绘制自己的静谧花园。她的对杯以当代的克制复现“鸡缸杯”的配色，藏家称之为“屏息的色彩”。',
      },
    },
  },
  {
    id: 4,
    glyph: '余',
    createdAt: '2025-03-06T09:00:00Z',
    translations: {
      enUS: {
        name: 'Yu Tiancheng',
        slug: 'yu-tiancheng',
        bio: 'Master potter Yu Tiancheng has thrown traditional forms on the wheel for forty-one years — meiping, general’s jars, hatstands. His hands remember proportions no drawing can record. State media call him “the living gauge of Jingdezhen.”',
      },
      zhCN: {
        name: '余天成',
        slug: 'yu-tiancheng',
        bio: '拉坯大师余天成在辘轳前坐了四十一年——梅瓶、将军罐、帽筒。他的双手记得图纸无法记载的比例。媒体称他为“景德镇的活尺寸”。',
      },
    },
  },
  {
    id: 5,
    glyph: '江',
    createdAt: '2025-03-08T09:00:00Z',
    translations: {
      enUS: {
        name: 'Jiang Xue',
        slug: 'jiang-xue',
        bio: 'A “Jingdrifter” who arrived with one backpack in 2019, Jiang Xue now makes sculptural vessels that quote Song silhouettes and city neon in the same breath. Her studio in Taoxichuan opens to visitors every first weekend of the month.',
      },
      zhCN: {
        name: '江雪',
        slug: 'jiang-xue',
        bio: '2019 年背着一个双肩包到来的“景漂”，江雪如今创作的雕塑器皿，既引用宋式轮廓，也呼应当代霓虹。她位于陶溪川的工作室每月首个周末对访客开放。',
      },
    },
  },
]

/* ------------------------------------------------------------------ */
/* Tags (taxonomy — canonical key + locale-resolved name)              */
/* ------------------------------------------------------------------ */

export const TAGS: Array<{ id: number; key: string; translations: Translation<string> }> = [
  { id: 1, key: 'qinghua', translations: { enUS: 'Qinghua blue & white', zhCN: '青花' } },
  { id: 2, key: 'celadon', translations: { enUS: 'Celadon', zhCN: '青瓷' } },
  { id: 3, key: 'enamel', translations: { enUS: 'Enamel color', zhCN: '彩瓷' } },
  { id: 9, key: 'fencai', translations: { enUS: 'Famille rose (fencai)', zhCN: '粉彩' } },
  { id: 10, key: 'yanseyou', translations: { enUS: 'Colored glaze', zhCN: '颜色釉' } },
  { id: 4, key: 'vase', translations: { enUS: 'Vases', zhCN: '瓶器' } },
  { id: 5, key: 'teaware', translations: { enUS: 'Teaware', zhCN: '茶器' } },
  { id: 6, key: 'tableware', translations: { enUS: 'Tableware', zhCN: '餐具' } },
  { id: 7, key: 'sculpture', translations: { enUS: 'Sculpture', zhCN: '雕塑' } },
  { id: 8, key: 'one-of-a-kind', translations: { enUS: 'One of a kind', zhCN: '孤品' } },
]

/* ------------------------------------------------------------------ */
/* Products                                                            */
/* ------------------------------------------------------------------ */

export const PRODUCTS: ProductRecord[] = [
  {
    id: 1,
    artistId: 1,
    category: 'vases',
    tags: ['qinghua', 'vase'],
    figureSeed: 101,
    figureKind: 'vase',
    featured: true,
    createdAt: '2026-05-12T08:00:00Z',
    publishedAt: '2026-05-14T08:00:00Z',
    translations: {
      enUS: {
        title: 'River Landscape Meiping',
        slug: 'river-landscape-meiping',
        description:
          'A meiping of classical proportion painted with a continuous river landscape — mountains, sail, and moon in stacked cobalt washes. Thrown by master Yu, painted by Chen Yuqing. Each edition is painted freehand, so every landscape differs in weather.',
        metaTitle: 'River Landscape Meiping — qinghua vase by Chen Yuqing',
        metaDescription:
          'Hand-painted blue-and-white meiping vase from Jingdezhen. Limited edition of 60, certified, shipped worldwide.',
      },
      zhCN: {
        title: '江山图梅瓶',
        slug: 'jiangshan-meiping',
        description:
          '经典比例梅瓶，通景绘江山——山峦、帆影与月色以浓淡三层青花分水写就。余天成拉坯，陈雨晴绘制。每版徒手作画，江山气象各有不同。',
        metaTitle: '江山图梅瓶 — 陈雨晴青花瓶',
        metaDescription: '景德镇手绘青花梅瓶。限量 60 件，附证书，全球直邮。',
      },
    },
    skus: [
      {
        id: 11,
        skuCode: 'MEIPING-RIVER-38',
        priceCny: 4280000,
        stock: 6,
        weightGrams: 2600,
        lowStockThreshold: 2,
        attributes: {
          enUS: {
            size: 'H 38 cm',
            technique: 'Underglaze blue (qinghua)',
            glaze: 'Clear glaze',
            edition_type: 'limited_edition',
            edition_number: '60',
            year: 2025,
            kiln: 'Xianghu Kiln',
          },
          zhCN: {
            size: '高 38 cm',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'limited_edition',
            edition_number: '60',
            year: 2025,
            kiln: '湘湖窑',
          },
        },
      },
      {
        id: 12,
        skuCode: 'MEIPING-RIVER-45',
        priceCny: 6880000,
        stock: 3,
        weightGrams: 3900,
        lowStockThreshold: 2,
        attributes: {
          enUS: {
            size: 'H 45 cm',
            technique: 'Underglaze blue (qinghua)',
            glaze: 'Clear glaze',
            edition_type: 'limited_edition',
            edition_number: '60',
            year: 2025,
            kiln: 'Xianghu Kiln',
          },
          zhCN: {
            size: '高 45 cm',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'limited_edition',
            edition_number: '60',
            year: 2025,
            kiln: '湘湖窑',
          },
        },
      },
    ],
  },
  {
    id: 2,
    artistId: 4,
    category: 'tableware',
    tags: ['qinghua', 'tableware'],
    figureSeed: 102,
    figureKind: 'bowl',
    featured: true,
    createdAt: '2026-04-02T08:00:00Z',
    publishedAt: '2026-04-05T08:00:00Z',
    translations: {
      enUS: {
        title: 'Lotus Scroll Bowl (set of 2)',
        slug: 'lotus-scroll-bowls',
        description:
          'Everyday bowls with an imperial pedigree: the lotus-scroll border is drawn from a Yongle-era pattern book, scaled for the modern table. Dishwasher-fearless, museum-adjacent.',
        metaTitle: 'Lotus Scroll Bowls — qinghua tableware set',
        metaDescription:
          'Set of two blue-and-white lotus bowls from Jingdezhen. Open edition, daily use, shipped worldwide.',
      },
      zhCN: {
        title: '缠枝莲纹碗（一对）',
        slug: 'lianhua-wan',
        description:
          '有御用血统的日常碗：莲纹边饰源自永乐纹样册页，按当代餐桌比例重绘。经得起洗碗机，也配得上展柜。',
        metaTitle: '缠枝莲纹碗 — 青花餐具',
        metaDescription: '景德镇青花莲纹碗一对。开放版，日用之器，全球直邮。',
      },
    },
    skus: [
      {
        id: 21,
        skuCode: 'BOWL-LOTUS-12',
        priceCny: 480000,
        stock: 24,
        weightGrams: 850,
        lowStockThreshold: 4,
        attributes: {
          enUS: {
            size: 'Ø 12.5 cm',
            technique: 'Underglaze blue',
            glaze: 'Clear glaze',
            edition_type: 'open_production',
            year: 2026,
            kiln: 'Xianghu Kiln',
          },
          zhCN: {
            size: '口径 12.5 cm',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'open_production',
            year: 2026,
            kiln: '湘湖窑',
          },
        },
      },
    ],
  },
  {
    id: 3,
    artistId: 1,
    category: 'plates',
    tags: ['qinghua', 'tableware'],
    figureSeed: 103,
    figureKind: 'plate',
    featured: true,
    createdAt: '2026-06-01T08:00:00Z',
    publishedAt: '2026-06-03T08:00:00Z',
    translations: {
      enUS: {
        title: 'Moon Plate — Rising Waves',
        slug: 'moon-plate-rising-waves',
        description:
          'A charger whose well boils with wave medallions under a full moon rim. Painted in the “heaped and piled” style of early Ming, when cobalt was precious and applied thick.',
        metaTitle: 'Moon Plate — qinghua charger by Chen Yuqing',
        metaDescription:
          'Blue-and-white charger plate with wave medallion. Limited edition 200 from Jingdezhen.',
      },
      zhCN: {
        title: '月华盘·海浪纹',
        slug: 'yuehua-pan',
        description:
          '折沿大盘，盘心海浪纹如沸，口沿一圈满月。以明初“铁锈斑”浓笔技法绘就——彼时钴料贵重，落笔厚重。',
        metaTitle: '月华盘 — 陈雨晴青花盘',
        metaDescription: '青花海浪纹折沿大盘。景德镇限量 200 件。',
      },
    },
    skus: [
      {
        id: 31,
        skuCode: 'PLATE-MOON-32',
        priceCny: 2180000,
        stock: 11,
        weightGrams: 1900,
        lowStockThreshold: 3,
        attributes: {
          enUS: {
            size: 'Ø 32 cm',
            technique: 'Underglaze blue, heaped & piled',
            glaze: 'Clear glaze',
            edition_type: 'limited_edition',
            edition_number: '200',
            year: 2026,
            kiln: 'Xianghu Kiln',
          },
          zhCN: {
            size: '口径 32 cm',
            technique: '釉下青花·浓笔堆料',
            glaze: '透明釉',
            edition_type: 'limited_edition',
            edition_number: '200',
            year: 2026,
            kiln: '湘湖窑',
          },
        },
      },
    ],
  },
  {
    id: 4,
    artistId: 4,
    category: 'teaware',
    tags: ['qinghua', 'teaware', 'one-of-a-kind'],
    figureSeed: 104,
    figureKind: 'teapot',
    featured: true,
    createdAt: '2026-07-08T08:00:00Z',
    publishedAt: '2026-07-10T08:00:00Z',
    translations: {
      enUS: {
        title: 'Cloud Teapot (one of a kind)',
        slug: 'cloud-teapot',
        description:
          'Thrown and painted by Yu Tiancheng for this gallery alone: a full-bodied pot carrying three auspicious clouds across its shoulder. One exists. When it sells, the provenance chain begins on its certificate.',
        metaTitle: 'Cloud Teapot — one-of-a-kind qinghua teapot',
        metaDescription:
          'Unique blue-and-white teapot by master potter Yu Tiancheng. One of one, certified.',
      },
      zhCN: {
        title: '祥云壶（孤品）',
        slug: 'xiangyun-hu',
        description:
          '余天成为本画廊独制：圆腹壶肩绘祥云三朵。世上仅此一件。售出之日起，证书上的流传记录随之开启。',
        metaTitle: '祥云壶 — 青花孤品茶壶',
        metaDescription: '拉坯大师余天成孤品青花壶。仅此一件，附证书。',
      },
    },
    skus: [
      {
        id: 41,
        skuCode: 'TEAPOT-CLOUD-UNIQUE',
        priceCny: 9880000,
        stock: 1,
        weightGrams: 1050,
        lowStockThreshold: 1,
        attributes: {
          enUS: {
            size: '480 ml',
            technique: 'Underglaze blue',
            glaze: 'Clear glaze',
            edition_type: 'one_of_a_kind',
            year: 2026,
            kiln: 'Yu Studio',
          },
          zhCN: {
            size: '480 ml',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'one_of_a_kind',
            year: 2026,
            kiln: '余氏工作室',
          },
        },
      },
    ],
  },
  {
    id: 5,
    artistId: 1,
    category: 'vases',
    tags: ['qinghua', 'vase'],
    figureSeed: 105,
    figureKind: 'jar',
    createdAt: '2026-03-15T08:00:00Z',
    publishedAt: '2026-03-18T08:00:00Z',
    translations: {
      enUS: {
        title: 'General’s Jar — Peony Crown',
        slug: 'generals-jar-peony',
        description:
          'The jiangjun guan takes its name from generals’ helmet crests. This one blooms with a peony medallion over a lotus-petal foot, cobalt deepening toward the base the way Kangxi jars do.',
        metaTitle: 'General’s Jar — qinghua jar with peony medallion',
        metaDescription:
          'Blue-and-white general’s jar from Jingdezhen. Limited edition, certified, worldwide shipping.',
      },
      zhCN: {
        title: '将军罐·牡丹',
        slug: 'jiangjun-guan',
        description:
          '将军罐因盖如将军盔顶而得名。此罐腹绘牡丹纹，胫部仰莲瓣，青花近底处渐深，一如康熙旧器。',
        metaTitle: '将军罐 — 青花牡丹纹罐',
        metaDescription: '景德镇青花将军罐。限量版，附证书，全球直邮。',
      },
    },
    skus: [
      {
        id: 51,
        skuCode: 'JAR-PEONY-30',
        priceCny: 5380000,
        stock: 4,
        weightGrams: 3100,
        lowStockThreshold: 2,
        attributes: {
          enUS: {
            size: 'H 30 cm',
            technique: 'Underglaze blue',
            glaze: 'Clear glaze',
            edition_type: 'limited_edition',
            edition_number: '80',
            year: 2025,
            kiln: 'Xianghu Kiln',
          },
          zhCN: {
            size: '高 30 cm',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'limited_edition',
            edition_number: '80',
            year: 2025,
            kiln: '湘湖窑',
          },
        },
      },
    ],
  },
  {
    id: 6,
    artistId: 2,
    category: 'sculpture',
    tags: ['celadon', 'sculpture', 'one-of-a-kind'],
    figureSeed: 106,
    figureKind: 'jar',
    featured: true,
    createdAt: '2026-02-20T08:00:00Z',
    publishedAt: '2026-02-25T08:00:00Z',
    translations: {
      enUS: {
        title: 'River Mist No. 7 (one of a kind)',
        slug: 'river-mist-no-7',
        description:
          'From Lin Haiming’s wood-fired “River Mist” series: a celadon form whose ash glaze pooled like dawn fog on the Chang. Fired 72 hours; the kiln signed the base where embers sat closest.',
        metaTitle: 'River Mist No. 7 — wood-fired celadon sculpture',
        metaDescription:
          'One-of-a-kind wood-fired celadon sculpture by Lin Haiming, Jingdezhen. Certified.',
      },
      zhCN: {
        title: '江雾·柒（孤品）',
        slug: 'jiangwu-7',
        description:
          '林海鸣柴烧“江雾”系列：青瓷之身，落灰釉如昌江晨雾积聚。七十二小时柴烧，窑火在近火处为底足留下了自己的签名。',
        metaTitle: '江雾·柒 — 柴烧青瓷雕塑',
        metaDescription: '林海鸣柴烧青瓷孤品，景德镇出品。附证书。',
      },
    },
    skus: [
      {
        id: 61,
        skuCode: 'MIST-7-UNIQUE',
        priceCny: 12880000,
        stock: 1,
        weightGrams: 5400,
        lowStockThreshold: 1,
        attributes: {
          enUS: {
            size: 'H 42 cm',
            technique: 'Wood-fired, ash glaze',
            glaze: 'Natural ash celadon',
            edition_type: 'one_of_a_kind',
            year: 2025,
            kiln: 'Sanbao wood kiln',
          },
          zhCN: {
            size: '高 42 cm',
            technique: '柴烧·落灰釉',
            glaze: '自然灰青釉',
            edition_type: 'one_of_a_kind',
            year: 2025,
            kiln: '三宝柴窑',
          },
        },
      },
    ],
  },
  {
    id: 7,
    artistId: 3,
    category: 'teaware',
    tags: ['enamel', 'fencai', 'teaware'],
    figureSeed: 107,
    figureKind: 'bowl',
    createdAt: '2026-06-18T08:00:00Z',
    publishedAt: '2026-06-20T08:00:00Z',
    translations: {
      enUS: {
        title: 'Quiet Garden Cups (pair)',
        slug: 'quiet-garden-cups',
        description:
          'Su Wanqing’s famille-rose cups carry a garden at dusk — one bloom opening, one about to. The translucent enamels sit on a white ground so clean it reads as moonlight.',
        metaTitle: 'Quiet Garden Cups — famille rose teacups pair',
        metaDescription:
          'Pair of famille-rose teacups by Su Wanqing, Jingdezhen. Open edition, certified.',
      },
      zhCN: {
        title: '静园对杯',
        slug: 'jingyuan-duibei',
        description:
          '苏婉清的粉彩对杯绘黄昏花园——一朵正开，一朵将开。透明彩料落于极净的白地之上，如月色一般。',
        metaTitle: '静园对杯 — 粉彩茶杯一对',
        metaDescription: '苏婉清粉彩对杯，景德镇出品。开放版，附证书。',
      },
    },
    skus: [
      {
        id: 71,
        skuCode: 'CUP-GARDEN-PAIR',
        priceCny: 980000,
        stock: 15,
        weightGrams: 480,
        lowStockThreshold: 3,
        attributes: {
          enUS: {
            size: '80 ml × 2',
            technique: 'Famille rose (fencai) enamel',
            glaze: 'White ground',
            edition_type: 'open_production',
            year: 2026,
            kiln: 'Su Studio',
          },
          zhCN: {
            size: '80 ml × 2',
            technique: '粉彩',
            glaze: '白地',
            edition_type: 'open_production',
            year: 2026,
            kiln: '苏氏工作室',
          },
        },
      },
    ],
  },
  {
    id: 8,
    artistId: 4,
    category: 'tableware',
    tags: ['qinghua', 'tableware'],
    figureSeed: 108,
    figureKind: 'plate',
    createdAt: '2026-01-25T08:00:00Z',
    publishedAt: '2026-02-01T08:00:00Z',
    translations: {
      enUS: {
        title: 'Dinner Service for Six — Willow Border',
        slug: 'dinner-service-willow',
        description:
          'Eighteen pieces of everyday imperial: dinner plates, side plates, and rice bowls wearing a willow border distilled from export-ware that sailed for Europe two centuries ago. Packed as one substantial crate.',
        metaTitle: 'Dinner Service for Six — qinghua tableware',
        metaDescription:
          'Eighteen-piece blue-and-white dinner set from Jingdezhen, packed for worldwide shipping.',
      },
      zhCN: {
        title: '六人餐具·柳纹边',
        slug: 'liuwen-canju',
        description:
          '十八件日常御风：餐盘、副盘与饭碗，绘两百年前远销欧洲的外销瓷柳纹精简而成。整箱发出，分量十足。',
        metaTitle: '六人青花餐具',
        metaDescription: '景德镇十八件青花餐具整箱装，全球直邮。',
      },
    },
    skus: [
      {
        id: 81,
        skuCode: 'SERVICE-WILLOW-18',
        priceCny: 7680000,
        stock: 8,
        weightGrams: 12500, // 12.5 kg — the overweight demo (exceeds some countries' top tier)
        lowStockThreshold: 2,
        attributes: {
          enUS: {
            size: '18 pieces',
            technique: 'Underglaze blue',
            glaze: 'Clear glaze',
            edition_type: 'open_production',
            year: 2026,
            kiln: 'Xianghu Kiln',
          },
          zhCN: {
            size: '18 件',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'open_production',
            year: 2026,
            kiln: '湘湖窑',
          },
        },
      },
    ],
  },
  {
    id: 9,
    artistId: 5,
    category: 'vases',
    tags: ['qinghua', 'vase', 'one-of-a-kind', 'sculpture'],
    figureSeed: 109,
    figureKind: 'vase',
    createdAt: '2026-07-01T08:00:00Z',
    publishedAt: '2026-07-03T08:00:00Z',
    translations: {
      enUS: {
        title: 'Sanbao Ridge (one of a kind)',
        slug: 'sanbao-ridge',
        description:
          'Jiang Xue’s skyline vase: the ridge above Sanbao valley at blue hour, its contour echoing a Song meiping while the skyline lights read as stamped neon. The collision is the point.',
        metaTitle: 'Sanbao Ridge — contemporary qinghua vase',
        metaDescription:
          'One-of-a-kind contemporary blue-and-white vase by Jingdrifter artist Jiang Xue.',
      },
      zhCN: {
        title: '三宝山脊（孤品）',
        slug: 'sanbao-shanji',
        description:
          '江雪的城市天际线瓶：蓝调时刻的三宝山脊，轮廓呼应宋式梅瓶，天际线灯光如钤印霓虹。碰撞即是主题。',
        metaTitle: '三宝山脊 — 当代青花瓶',
        metaDescription: '景漂艺术家江雪当代青花孤品一件。',
      },
    },
    skus: [
      {
        id: 91,
        skuCode: 'SANBAO-RIDGE-UNIQUE',
        priceCny: 8880000,
        stock: 1,
        weightGrams: 2200,
        lowStockThreshold: 1,
        attributes: {
          enUS: {
            size: 'H 35 cm',
            technique: 'Underglaze blue, decal neon',
            glaze: 'Satin clear',
            edition_type: 'one_of_a_kind',
            year: 2026,
            kiln: 'Taoxichuan studio',
          },
          zhCN: {
            size: '高 35 cm',
            technique: '釉下青花·霓虹贴花',
            glaze: '亚光釉',
            edition_type: 'one_of_a_kind',
            year: 2026,
            kiln: '陶溪川工作室',
          },
        },
      },
    ],
  },
  {
    id: 10,
    artistId: 2,
    category: 'teaware',
    tags: ['celadon', 'linglong', 'teaware'],
    figureSeed: 110,
    figureKind: 'bowl',
    createdAt: '2026-05-05T08:00:00Z',
    publishedAt: '2026-05-08T08:00:00Z',
    translations: {
      enUS: {
        title: 'Kiln-Changed Tea Bowls (set of 2)',
        slug: 'kiln-changed-bowls',
        description:
          'No two firings agree. These paired bowls went into the kiln identical and came out cousins — one holding a lake-green flash, one a honey edge where the flame turned. Glaze chemistry as a two-act play.',
        metaTitle: 'Kiln-Changed Tea Bowls — yōhen celadon pair',
        metaDescription: 'Pair of kiln-transmutation celadon tea bowls by Lin Haiming, Jingdezhen.',
      },
      zhCN: {
        title: '窑变茶盏（一对）',
        slug: 'yaobian-zhan',
        description:
          '入窑一色，出窑万彩。这对茶盏同坯同釉，出窑成了表亲——一只湖水绿，一只蜜色镶边。釉的化学，两幕剧。',
        metaTitle: '窑变茶盏 — 青瓷一对',
        metaDescription: '林海鸣窑变青瓷茶盏一对，景德镇出品。',
      },
    },
    skus: [
      {
        id: 101,
        skuCode: 'BOWL-KILNCHANGE-2',
        priceCny: 680000,
        stock: 9,
        weightGrams: 520,
        lowStockThreshold: 2,
        attributes: {
          enUS: {
            size: '110 ml × 2',
            technique: 'Reduction-fired celadon',
            glaze: 'Kiln-transmutation',
            edition_type: 'limited_edition',
            edition_number: '150',
            year: 2026,
            kiln: 'Sanbao wood kiln',
          },
          zhCN: {
            size: '110 ml × 2',
            technique: '还原焰青瓷',
            glaze: '窑变釉',
            edition_type: 'limited_edition',
            edition_number: '150',
            year: 2026,
            kiln: '三宝柴窑',
          },
        },
      },
    ],
  },
  {
    id: 11,
    artistId: 3,
    category: 'vases',
    tags: ['enamel', 'yanseyou', 'vase'],
    figureSeed: 111,
    figureKind: 'jar',
    createdAt: '2026-04-22T08:00:00Z',
    publishedAt: '2026-04-26T08:00:00Z',
    translations: {
      enUS: {
        title: 'Imperial Yellow Ginger Jar',
        slug: 'imperial-yellow-jar',
        description:
          'The yellow reserved for emperors, poured over a ribbed jar and closed with a domed lid. Su Wanqing fires it only in short winter windows, when the kiln cools slowly enough to keep the color tender.',
        metaTitle: 'Imperial Yellow Ginger Jar — enamel porcelain',
        metaDescription:
          'Imperial-yellow enamel ginger jar by Su Wanqing, Jingdezhen. Open edition.',
      },
      zhCN: {
        title: '明黄地瓜楞罐',
        slug: 'minghuang-guan',
        description:
          '帝王专属的黄，罩在瓜楞罐身，覆一穹顶盖。苏婉清只在冬季短窗烧制——降温足够慢，颜色才足够娇。',
        metaTitle: '明黄地瓜楞罐 — 彩瓷',
        metaDescription: '苏婉清明黄釉瓜楞罐，景德镇出品。开放版。',
      },
    },
    skus: [
      {
        id: 111,
        skuCode: 'JAR-YELLOW-18',
        priceCny: 1880000,
        stock: 13,
        weightGrams: 1600,
        lowStockThreshold: 3,
        attributes: {
          enUS: {
            size: 'H 18 cm',
            technique: 'Lead-glaze enamel',
            glaze: 'Imperial yellow',
            edition_type: 'open_production',
            year: 2025,
            kiln: 'Su Studio',
          },
          zhCN: {
            size: '高 18 cm',
            technique: '低温釉彩',
            glaze: '明黄',
            edition_type: 'open_production',
            year: 2025,
            kiln: '苏氏工作室',
          },
        },
      },
    ],
  },
  {
    id: 12,
    artistId: 5,
    category: 'tableware',
    tags: ['qinghua', 'tableware'],
    figureSeed: 112,
    figureKind: 'teapot',
    createdAt: '2026-06-28T08:00:00Z',
    publishedAt: '2026-07-01T08:00:00Z',
    translations: {
      enUS: {
        title: 'Comma Cups (set of 4)',
        slug: 'comma-cups',
        description:
          'Jiang Xue’s first tableware line: four cups, each struck with a single cobalt comma — the brushstroke before a sentence. A quiet joke about how every conversation over tea begins.',
        metaTitle: 'Comma Cups — qinghua cups set of four',
        metaDescription: 'Set of four contemporary blue-and-white cups by Jiang Xue, Jingdezhen.',
      },
      zhCN: {
        title: '逗号杯（四只）',
        slug: 'douhao-bei',
        description:
          '江雪的首套餐具：四只杯，各落一笔试青花逗号——句子开始前的那一笔。关于茶叙如何开始的安静玩笑。',
        metaTitle: '逗号杯 — 青花杯四只',
        metaDescription: '江雪当代青花杯四只，景德镇出品。',
      },
    },
    skus: [
      {
        id: 121,
        skuCode: 'CUP-COMMA-4',
        priceCny: 580000,
        stock: 30,
        weightGrams: 720,
        lowStockThreshold: 5,
        attributes: {
          enUS: {
            size: '90 ml × 4',
            technique: 'Underglaze blue',
            glaze: 'Clear glaze',
            edition_type: 'open_production',
            year: 2026,
            kiln: 'Taoxichuan studio',
          },
          zhCN: {
            size: '90 ml × 4',
            technique: '釉下青花',
            glaze: '透明釉',
            edition_type: 'open_production',
            year: 2026,
            kiln: '陶溪川工作室',
          },
        },
      },
    ],
  },
]

/* ------------------------------------------------------------------ */
/* Ceramic stories (History & Heritage)                                */
/* ------------------------------------------------------------------ */

export const STORIES: StoryRecord[] = [
  {
    id: 1,
    startYear: 1004,
    figureSeed: 201,
    translations: {
      enUS: {
        title: 'The Town the Song Named',
        slug: 'town-the-song-named',
        summary:
          'In 1004, an emperor’s reign name — Jingde — was given to a river town already firing translucent ware. The rest is porcelain history.',
        content: [
          {
            type: 'paragraph',
            text: 'When the Song emperor Zhenzong decreed in 1004 that the town of Changnanzhen should stamp his reign name — Jingde — on the bases of its wares, he was registering a brand. The town kept the name and outlived the dynasty.',
          },
          {
            type: 'paragraph',
            text: 'What earned the decree was qingbai: “shadow blue” porcelain so translucent that bowls were said to hold light the way paper lanterns do. Kilns lined the Chang river for miles, and the river itself became the supply chain — kaolin washed downstream from the hills, crates of fired ware floated out to the world.',
          },
          { type: 'heading', level: 2, text: 'Why here' },
          {
            type: 'paragraph',
            text: 'Two clays made the miracle: plastic kaolin and stone-rich porcelain stone. Mixed in the right ratio, they could be thrown thin, survive 1,300°C, and ring when struck. Jingdezhen sat on both, with pine forests for fuel and a river for transport. Geography, fired into an industry.',
          },
        ],
      },
      zhCN: {
        title: '以年号为名的镇子',
        slug: 'nianhao-zhichen',
        summary:
          '公元 1004 年，皇帝的年号“景德”被赐给一座已能烧制透光瓷器的河畔小镇。此后的历史，就是瓷器的历史。',
        content: [
          {
            type: 'paragraph',
            text: '宋真宗景德元年，下旨将昌南镇所制瓷器底书“景德年制”——这几乎是一次“御用品牌注册”。王朝早已更迭，小镇却把年号留成了自己的名字。',
          },
          {
            type: 'paragraph',
            text: '获此殊荣的是青白瓷：“影青”透光如纸灯笼盛光。窑火沿昌江绵延数十里，江水即是供应链——高岭自山中淘洗而下，成器的木箱顺流漂向世界。',
          },
          { type: 'heading', level: 2, text: '为什么是这里' },
          {
            type: 'paragraph',
            text: '两种土成就了奇迹：可塑的高岭土与瓷石。以恰当配比混合，即可拉薄坯、耐一千三百度高温、叩之有声。景德镇坐拥二者，又有松柴为薪、江水为路。地理，被烧成了一个产业。',
          },
        ],
      },
    },
  },
  {
    id: 2,
    startYear: 1320,
    figureSeed: 202,
    translations: {
      enUS: {
        title: 'Cobalt Arrives: the Yuan Invention',
        slug: 'cobalt-arrives-yuan',
        summary:
          'Persian cobalt met Chinese clay on Yuan trade routes — and qinghua, blue-and-white, was born of the encounter.',
        content: [
          {
            type: 'paragraph',
            text: 'Under the Yuan, caravans and sea lanes brought “Sumali blue” — cobalt oxide from Persian mines — to Jingdezhen’s kilns. Painted under the glaze onto porcelain and fired once, it produced a blue that never faded, never washed, never wore.',
          },
          {
            type: 'paragraph',
            text: 'The first great blue-and-whites were export ware: enormous dishes made to Islamic specifications, their dense scrollwork mirroring Persian metalwork. Chinese potters learned the taste of distant customers and invented a visual language the whole world would claim as its own.',
          },
          { type: 'heading', level: 2, text: 'A trade glyph' },
          {
            type: 'paragraph',
            text: 'Within a century, qinghua was being copied in Vietnam, imitated in Persia, coveted in Istanbul, and painted onto Dutch delft. The blue had become the first global image — recognizable from the Strait of Malacca to the canals of Amsterdam.',
          },
        ],
      },
      zhCN: {
        title: '苏麻离青：元的发明',
        slug: 'sumali-qing',
        summary: '波斯的钴料沿元代商路抵达中国的瓷土——青花，诞生于这场相遇。',
        content: [
          {
            type: 'paragraph',
            text: '元代，商队与海路将“苏麻离青”——波斯矿藏所出的氧化钴——带入景德镇窑口。以釉下彩绘于瓷坯，一次烧成，其蓝永不褪色、不磨损、不流散。',
          },
          {
            type: 'paragraph',
            text: '最早的青花大器皆为外销：硕大的盘子按伊斯兰世界的定制烧造，繁密的缠枝纹映照着波斯金银器的趣味。景德镇的窑工学会了远方顾客的审美，并由此发明了被全世界认领的视觉语言。',
          },
          { type: 'heading', level: 2, text: '贸易的符号' },
          {
            type: 'paragraph',
            text: '不过百年，青花已被越南仿制、波斯效法、伊斯坦布尔珍藏，并绘上荷兰代尔夫特的陶器。这抹蓝，成了第一张全球通行的图像——从马六甲海峡到阿姆斯特丹运河，无人不识。',
          },
        ],
      },
    },
  },
  {
    id: 3,
    startYear: 1368,
    figureSeed: 203,
    translations: {
      enUS: {
        title: 'Imperial Kilns of the Ming',
        slug: 'imperial-kilns-ming',
        summary:
          'The Ming turned Jingdezhen into a state factory: official kilns, secret recipes, and the porcelain that defined “China” abroad.',
        content: [
          {
            type: 'paragraph',
            text: 'The Ming founded the Imperial Porcelain Factory at Zhushan — Pear Hill — in the heart of town. Its rejects were smashed and buried so that flawed imperial ware could never circulate; archaeologists now read those shards like minutes of court taste.',
          },
          {
            type: 'paragraph',
            text: 'This was porcelain as administration: quotas of plates for the palace, altar vessels for the temples, diplomatic gifts calibrated to the rank of the receiving envoy. The Yongle and Xuande eras perfected the thick “heaped and piled” cobalt; Chenghua’s tiny chicken cups became, five centuries on, the most expensive ceramics ever sold at auction.',
          },
          { type: 'heading', level: 2, text: 'The 72 hands' },
          {
            type: 'paragraph',
            text: 'Ming writers counted seventy-two separate trades in the town — clay washers, throwers, trimmers, painters of outlines, painters of washes, kiln watchers who could read flame color like text. The division of labor was the real invention: porcelain as a system, not a craft.',
          },
        ],
      },
      zhCN: {
        title: '明代御器厂',
        slug: 'mingde-yuqichang',
        summary: '明朝将景德镇变成国家工厂：御窑、秘方，以及定义了海外“中国”意象的瓷器。',
        content: [
          {
            type: 'paragraph',
            text: '明廷在镇中心的珠山设立御器厂。落选的贡品须砸碎掩埋，不得流入民间——考古学者如今像读宫廷品味纪要一样研读那些碎瓷片。',
          },
          {
            type: 'paragraph',
            text: '这是作为行政的制瓷：宫殿的配额用盘、祭坛的礼器、按受赠使节品级校准的国礼。永乐宣德的浓笔堆料登峰造极；成化的鸡缸杯在五百年后成为拍卖史上最贵的瓷器。',
          },
          { type: 'heading', level: 2, text: '过手七十二' },
          {
            type: 'paragraph',
            text: '明人记载镇上有七十二行当——淘泥的、拉坯的、利坯的、画线的、分水的，还有能将火焰颜色读成文章的看火师傅。分工协作才是真正的发明：瓷器作为一个系统，而非一门手艺。',
          },
        ],
      },
    },
  },
  {
    id: 4,
    startYear: 1949,
    figureSeed: 204,
    translations: {
      enUS: {
        title: 'Fall, State Factories, and the Jingdrifters',
        slug: 'jingdrifters-revival',
        summary:
          'From planned-economy porcelain mills to today’s river of young artists — how Jingdezhen became, again, a place people drift toward.',
        content: [
          {
            type: 'paragraph',
            text: 'The Qing court fell, imperial kilns closed, and war scattered the trades. In the 1950s the state reorganized everything into numbered porcelain factories — quality stayed formidable, export contracts kept kilns hot, but the seventy-two trades were consolidated into assembly lines.',
          },
          {
            type: 'paragraph',
            text: 'When the factories privatized in the 1990s, the town looked finished. Instead, something unexpected: young graduates arrived, renting cheap studios by the river. They are the “Jingdrifters” — 景漂 — now tens of thousands strong, throwing, painting, failing, and occasionally inventing the next chapter.',
          },
          { type: 'heading', level: 2, text: 'Today' },
          {
            type: 'paragraph',
            text: 'Sculptors, enamel painters, teapot throwers, and performance artists share supply chains with makers of dentist-grade kaolin. The ecosystem that once served emperors now serves anyone with an idea and a kiln slot — including the artists in this gallery.',
          },
        ],
      },
      zhCN: {
        title: '国营瓷厂与景漂',
        slug: 'guoying-yu-jingpiao',
        summary: '从计划经济的瓷厂到今天涌向此地的年轻艺术家——景德镇如何再次成为人们“漂”向的地方。',
        content: [
          {
            type: 'paragraph',
            text: '清廷倾覆，御窑停烧，战乱驱散了各行当。上世纪五十年代，国家将一切重组为编号瓷厂——品质依然可观，出口订单窑火不熄，但七十二行被并入了流水线。',
          },
          {
            type: 'paragraph',
            text: '九十年代工厂改制，小镇看似落幕。然而意想不到的事发生了：年轻人来了，在江边租下便宜的工作室。他们就是“景漂”——如今数以万计，拉坯、绘制、失败，也偶尔发明出下一个篇章。',
          },
          { type: 'heading', level: 2, text: '今天' },
          {
            type: 'paragraph',
            text: '雕塑家、粉彩画师、制壶师与行为艺术家，与生产牙科级高岭土的厂商共享同一条供应链。曾为帝王服务的生态，如今服务于任何一个有想法、有窑位的人——包括本画廊的艺术家们。',
          },
        ],
      },
    },
  },
]

/* ------------------------------------------------------------------ */
/* Engage: destinations & local lifestyle                              */
/* ------------------------------------------------------------------ */

export const ACTIVITIES: ActivityRecord[] = [
  {
    id: 1,
    type: 'destination',
    figureSeed: 301,
    lat: 29.2668,
    lng: 117.1976,
    address: {
      enUS: '103 Cidu Avenue, Zhushan District, Jingdezhen',
      zhCN: '景德镇市珠山区瓷都大道103号',
    },
    opening: {
      enUS: 'Museum Tue–Sun 9:00–17:00 · Avenue always open',
      zhCN: '博物馆 周二至周日 9:00–17:00 · 街区全天开放',
    },
    translations: {
      enUS: {
        title: 'Taoxichuan Ceramic Art Avenue',
        slug: 'taoxichuan',
        summary:
          'Old factory kilns reborn as studios, markets, and night life — the beating heart of the Jingdrifter scene.',
        content: [
          {
            type: 'paragraph',
            text: 'Taoxichuan (“creek flowing toward porcelain”) occupies the campus of the former Universal Porcelain Factory. Its preserved ring kilns and chimneys now frame design studios, glass galleries, and a weekend market where two hundred young potters sell direct.',
          },
          {
            type: 'paragraph',
            text: 'Come at dusk on a Friday: the market lights come on, the wood-fired pizza of the kiln courtyard meets the smell of fresh bisque, and somewhere a band is tuning beside a 1960s chimney. It is the friendliest possible introduction to contemporary Jingdezhen.',
          },
          {
            type: 'paragraph',
            text: 'The October “Spring Festival of Porcelain” fills the avenue with collectors; in ordinary weeks, you can commission a custom tea set over coffee and pick it up fired before you fly home.',
          },
        ],
      },
      zhCN: {
        title: '陶溪川陶瓷文化创意园',
        slug: 'taoxichuan',
        summary: '老窑厂重生为工作室、市集与夜生活——景漂现场的心脏。',
        content: [
          {
            type: 'paragraph',
            text: '陶溪川，取“溪水常流、陶源致远”之意，坐落于此前的宇宙瓷厂厂区。保留下来的环形窑与烟囱之间，如今是设计工作室、玻璃展厅，以及两百多位年轻陶作者直营的周末市集。',
          },
          {
            type: 'paragraph',
            text: '周五黄昏最宜前来：市集灯亮起，窑院里的柴火披萨混着新出窑的素坯气息，某支乐队正在六十年代的烟囱旁调音。这是认识当代景德镇最友好的方式。',
          },
          {
            type: 'paragraph',
            text: '十月“瓷博会”期间整条大道人头攒动；寻常日子里，你可以就着一杯咖啡定制一套茶器，赶在回国前取到烧成的成品。',
          },
        ],
      },
    },
  },
  {
    id: 2,
    type: 'destination',
    figureSeed: 302,
    lat: 29.1867,
    lng: 117.2436,
    address: {
      enUS: 'Sanbao Village, Jingdezhen (7 km southeast of the city)',
      zhCN: '景德镇市三宝村（市区东南 7 公里）',
    },
    opening: {
      enUS: 'Studios vary · valley walks always open',
      zhCN: '各工作室时间不一 · 山谷步道全天开放',
    },
    translations: {
      enUS: {
        title: 'Sanbao International Ceramic Valley',
        slug: 'sanbao-valley',
        summary:
          'A wooded valley of studios, wood kilns, and tea houses — where the mist and the clay come from.',
        content: [
          {
            type: 'paragraph',
            text: 'Sanbao is the porcelain heart behind the city: a narrow valley of paddies and workshops where some of the world’s best ceramic artists keep studios among the tea bushes. Wood kilns fire here on weekends; the smoke reads as weather.',
          },
          {
            type: 'paragraph',
            text: 'Walk the valley road past the Sanbao Ceramic Art Museum’s rammed-earth walls, call ahead at private studios, and end with tea where the kaolin was first mined. Most gallery artists here will host a visit if you write first.',
          },
          {
            type: 'paragraph',
            text: 'Our custom itineraries include hands-on sessions in Sanbao studios — see Custom Travel.',
          },
        ],
      },
      zhCN: {
        title: '三宝国际瓷谷',
        slug: 'sanbao-gu',
        summary: '一条布满工作室、柴窑与茶馆的林间山谷——雾与瓷土的来处。',
        content: [
          {
            type: 'paragraph',
            text: '三宝是城市背后的瓷心：一条由稻田与工坊组成的窄谷，世界一流的陶瓷艺术家在茶树间设有工作室。柴窑周末点火，烟即天气。',
          },
          {
            type: 'paragraph',
            text: '沿谷中道路走过三宝蓬美术馆的夯土墙，预约叩访私人工作室，最后在最初开采高岭土的地方坐下喝茶。提前致函，本画廊多数艺术家都愿意接待来访。',
          },
          { type: 'paragraph', text: '我们的定制行程包含三宝工作室的手作课程——见“定制旅行”。' },
        ],
      },
    },
  },
  {
    id: 3,
    type: 'destination',
    figureSeed: 303,
    lat: 29.2761,
    lng: 117.2056,
    address: {
      enUS: '1 Zhushan Middle Road, Zhushan District (inside the Imperial Kiln Site)',
      zhCN: '珠山区珠山中路1号（御窑厂遗址内）',
    },
    opening: {
      enUS: 'Tue–Sun 9:00–17:00, closed Mondays',
      zhCN: '周二至周日 9:00–17:00，周一闭馆',
    },
    translations: {
      enUS: {
        title: 'Imperial Kiln Museum',
        slug: 'imperial-kiln-museum',
        summary:
          'Eight brick vaults sunk into the imperial kiln site, holding the shattered evidence of five centuries of court taste.',
        content: [
          {
            type: 'paragraph',
            text: 'Built over the Ming Imperial Porcelain Factory, the museum’s interlocking vaults — half-buried, kiln-shaped — hold the reconstructed fragments of porcelain destroyed for imperfection four centuries before anyone could study them.',
          },
          {
            type: 'paragraph',
            text: 'The permanent display walks from Yongle blue to Chenghua enamel; the repair studio, behind glass, shows conservators reassembling a Xuande bowl from four hundred shards. The building itself won international awards before it opened.',
          },
        ],
      },
      zhCN: {
        title: '景德镇御窑博物馆',
        slug: 'yuyao-bowuguan',
        summary: '八座下沉于御窑遗址的砖拱，收藏着五个世纪宫廷品味的碎瓷证据。',
        content: [
          {
            type: 'paragraph',
            text: '博物馆建于明代御器厂遗址之上，半埋于地下的连环拱体本身就是窑的形状——收藏着四个世纪前因瑕疵被毁、无人得以研究的瓷器重拼之作。',
          },
          {
            type: 'paragraph',
            text: '常设展自永乐青花走到成化斗彩；玻璃之后的修复室里，文物修复师正用四百片碎瓷拼合一只宣德碗。建筑本身在开馆前便已屡获国际大奖。',
          },
        ],
      },
    },
  },
  {
    id: 4,
    type: 'destination',
    figureSeed: 304,
    lat: 29.3111,
    lng: 117.1711,
    address: {
      enUS: 'Ancient Kiln Folk Customs Museum, Fengshuwan, Jingdezhen',
      zhCN: '景德镇枫树山古窑民俗博览区',
    },
    opening: { enUS: 'Daily 8:00–17:30', zhCN: '每日 8:00–17:30' },
    translations: {
      enUS: {
        title: 'Ancient Kiln Folk Customs Park',
        slug: 'ancient-kiln-park',
        summary:
          'Working reconstructions of Ming kilns and the seventy-two trades — porcelain as live performance.',
        content: [
          {
            type: 'paragraph',
            text: 'The park keeps the old trades employable: throwers at Ming-style wheels, a zhen qiqiao kiln fired on festival days, craftsmen demonstrating each of the seventy-two processes with their original tools.',
          },
          {
            type: 'paragraph',
            text: 'Children press their own bowls; adults argue with the glaze chemist. The twice-daily firing demonstrations are the closest a visitor gets to the heat that made the town.',
          },
        ],
      },
      zhCN: {
        title: '古窑民俗博览区',
        slug: 'guyao-minsu',
        summary: '明代窑炉与七十二道工序的活态复原——瓷器作为现场演出。',
        content: [
          {
            type: 'paragraph',
            text: '园区让老行当持续在场：明式辘轳前的拉坯师傅、节庆点火的镇窑匠人，以及以原工具演示七十二道工序的手艺人。',
          },
          {
            type: 'paragraph',
            text: '孩子们可以压一只自己的碗，大人们会与配釉师傅争论配方。每日两场的复烧演示，是游客离造就这座城的窑火最近的一刻。',
          },
        ],
      },
    },
  },
  {
    id: 5,
    type: 'lifestyle',
    figureSeed: 305,
    translations: {
      enUS: {
        title: 'A Day in the Life of a Jingdrifter',
        slug: 'jingdrifter-day',
        summary:
          'Morning market clay, afternoon commissions, night kiln gossip — the rhythm that pulls graduates back to the river town.',
        content: [
          {
            type: 'paragraph',
            text: '6:40, the material market: buckets of slip, bats of leather-hard clay, boxes of cobalt priced by the gram. 9:00, studio: commissions first — the teapot order for a Berlin shop pays this month’s kiln slot. 14:00, painting window, when the light is flat and the brush behaves.',
          },
          {
            type: 'paragraph',
            text: 'Evening is professional development: someone’s firing tonight, and firing night is open house. Pizza appears, guitars appear, and around the kiln the town’s real economy — gossip, technique, and who got into which fair — is transacted.',
          },
          {
            type: 'paragraph',
            text: 'Why stay? “In Beijing I was a designer,” one drift veteran says, gesturing at the valley. “Here, I’m a potter. The town decides what you are — and it always says potter.”',
          },
        ],
      },
      zhCN: {
        title: '景漂的一天',
        slug: 'jingpiao-de-yitian',
        summary: '早市买泥、午后赶单、夜里窑边长谈——把毕业生一次次拉回这座河城的节奏。',
        content: [
          {
            type: 'paragraph',
            text: '6:40，原料市场：一桶桶泥浆、一板板皮革硬度的坯、论克标价的钴料。9:00，工作室：先赶单——柏林茶具店的那批壶，付得起这个月的窑位。14:00，绘制时段，光线平了，笔才听话。',
          },
          {
            type: 'paragraph',
            text: '晚上是“专业进修”：今晚有人开窑，开窑即开放日。披萨会有的，吉他也会有的，而窑边进行的，是这座城真正的经济——八卦、技法，以及谁又进了哪个市集。',
          },
          {
            type: 'paragraph',
            text: '为什么留下？“在北京我是设计师，”一位老景漂指着山谷说，“在这儿，我是陶作者。这座城决定你是谁——它永远回答：陶作者。”',
          },
        ],
      },
    },
  },
  {
    id: 6,
    type: 'lifestyle',
    figureSeed: 306,
    translations: {
      enUS: {
        title: 'Opening the Kiln: Notes from Firing Night',
        slug: 'opening-the-kiln',
        summary:
          'Seventy-two hours of fire, three days of cooling, ninety seconds of truth — inside a wood firing with Lin Haiming.',
        content: [
          {
            type: 'paragraph',
            text: 'A wood firing is a wager with weather. The kiln wants thirty-six hours at climbing temperature, stoked every four minutes by rotating crews; rain dampens the wood; wind chooses where the ash lands. Lin Haiming logs everything in a school notebook, in pencil.',
          },
          {
            type: 'paragraph',
            text: 'Cooling takes longer than firing. On the third morning the bricks come out one course at a time — nobody speaks — and the first wares emerge holding whatever the flame decided: a jade shoulder, a honeyed rim, occasionally a miracle, occasionally a lesson.',
          },
          {
            type: 'paragraph',
            text: 'Pieces from these firings reach this gallery marked with the date of the opening. That date is the piece’s second birthday; the kiln keeps the first.',
          },
        ],
      },
      zhCN: {
        title: '开窑：柴烧夜记',
        slug: 'kaiyao-ji',
        summary: '七十二小时火、三天冷却、九十秒的真相——随林海鸣走进一次柴烧。',
        content: [
          {
            type: 'paragraph',
            text: '柴烧是与天气的对赌。窑要三十六小时的爬温，每四分钟一次投柴，轮班不歇；雨会让柴返潮，风决定灰落在哪里。这一切，林海鸣都用铅笔记在一本练习簿上。',
          },
          {
            type: 'paragraph',
            text: '冷却比烧成更久。第三天清晨，窑砖一层层拆下——没人说话——最初取出的器物，带着火焰的全部裁决：玉色的肩、蜜色的边，偶有奇迹，偶有教训。',
          },
          {
            type: 'paragraph',
            text: '这些窑次的作品来到本画廊时，都标注着开窑日期。那是作品的第二个生日；第一个，窑自己收着。',
          },
        ],
      },
    },
  },
]

/* ------------------------------------------------------------------ */
/* Users / addresses / orders / itineraries / wishlist (seed state)    */
/* ------------------------------------------------------------------ */

export const DEMO_USERS = [
  {
    id: 'u_9f3a',
    email: 'emily@demo.dev',
    password: 'porcelain123',
    twoFA: false,
    user: {
      id: 'u_9f3a',
      email: 'emily@demo.dev',
      nickname: 'Emily Zhang',
      avatar_glyph: 'E',
      role: 'customer' as const,
      preferred_locale: 'en-US',
      preferred_currency: 'USD',
      created_at: '2025-11-03T10:00:00Z',
    },
  },
  {
    id: 'u_0001',
    email: 'admin@demo.dev',
    password: 'porcelain123',
    twoFA: true,
    demoCode: '123456',
    user: {
      id: 'u_0001',
      email: 'admin@demo.dev',
      nickname: 'Platform Admin',
      avatar_glyph: '印',
      role: 'super_admin' as const,
      roles: ['super_admin' as const],
      preferred_locale: 'en-US',
      preferred_currency: 'USD',
      created_at: '2025-01-01T00:00:00Z',
    },
  },
  {
    id: 'u_0002',
    email: 'editor@demo.dev',
    password: 'porcelain123',
    twoFA: false,
    user: {
      id: 'u_0002',
      email: 'editor@demo.dev',
      nickname: 'Content Editor',
      avatar_glyph: '编',
      role: 'content_editor' as const,
      roles: ['content_editor' as const],
      preferred_locale: 'en-US',
      preferred_currency: 'USD',
      created_at: '2025-03-15T00:00:00Z',
    },
  },
]

export const SEED_ADDRESSES = [
  {
    id: 1,
    user_id: 'u_9f3a',
    recipient: 'Emily Zhang',
    line1: '1234 NW Quimby Street',
    line2: 'Apt 5B',
    city: 'Portland',
    region: 'OR',
    postal_code: '97209',
    country: 'US',
    phone: '+1 503 555 0134',
    is_default: true,
  },
  {
    id: 2,
    user_id: 'u_9f3a',
    recipient: 'Emily Zhang',
    line1: '21 Charlotte Road',
    city: 'London',
    region: '',
    postal_code: 'EC2A 3PD',
    country: 'GB',
    phone: '+44 20 7946 0812',
    is_default: false,
  },
]

export const SEED_ORDERS = [
  {
    id: 1001,
    user_id: 'u_9f3a',
    status: 'paid' as const,
    currency: 'USD',
    subtotal_minor: 310900,
    shipping_minor: 35400,
    total_minor: 346300,
    subtotal_cny: 2180000,
    shipping_cny: 248000,
    total_cny: 2428000,
    fx_rate_used: 7.017,
    address_id: 1,
    locale: 'en-US',
    placed_at: '2026-08-09T14:32:00Z',
    paid_at: '2026-08-09T14:35:12Z',
    items: [
      {
        id: 9001,
        order_id: 1001,
        sku_id: 31,
        qty: 1,
        unit_price_minor: 310900,
        unit_price_cny: 2180000,
        title_snapshot: { enUS: 'Moon Plate — Rising Waves', zhCN: '月华盘·海浪纹' },
        figure_seed: 103,
        figure_kind: 'plate' as const,
      },
    ],
  },
  {
    id: 1002,
    user_id: 'u_9f3a',
    status: 'shipped' as const,
    currency: 'GBP',
    subtotal_minor: 186200,
    shipping_minor: 188500,
    total_minor: 374700,
    subtotal_cny: 1660000,
    shipping_cny: 168000,
    total_cny: 1828000,
    fx_rate_used: 8.917,
    address_id: 2,
    locale: 'en-US',
    carrier_name: 'DHL Express',
    tracking_number: 'JDZ0044771198',
    placed_at: '2026-07-22T09:12:00Z',
    paid_at: '2026-07-22T09:14:44Z',
    shipped_at: '2026-07-25T16:40:00Z',
    items: [
      {
        id: 9002,
        order_id: 1002,
        sku_id: 71,
        qty: 1,
        unit_price_minor: 110200,
        unit_price_cny: 980000,
        title_snapshot: { enUS: 'Quiet Garden Cups (pair)', zhCN: '静园对杯' },
        figure_seed: 107,
        figure_kind: 'bowl' as const,
      },
      {
        id: 9003,
        order_id: 1002,
        sku_id: 101,
        qty: 1,
        unit_price_minor: 76000,
        unit_price_cny: 680000,
        title_snapshot: { enUS: 'Kiln-Changed Tea Bowls (set of 2)', zhCN: '窑变茶盏（一对）' },
        figure_seed: 110,
        figure_kind: 'bowl' as const,
      },
    ],
  },
  {
    id: 1003,
    user_id: 'u_9f3a',
    status: 'completed' as const,
    currency: 'EUR',
    subtotal_minor: 322300,
    shipping_minor: 256800,
    total_minor: 579100,
    subtotal_cny: 2460000,
    shipping_cny: 196000,
    total_cny: 2656000,
    fx_rate_used: 7.633,
    address_id: 1,
    locale: 'en-US',
    carrier_name: 'SF Express',
    tracking_number: 'SF7712003345US',
    placed_at: '2026-05-30T11:02:00Z',
    paid_at: '2026-05-30T11:05:00Z',
    shipped_at: '2026-06-02T09:30:00Z',
    completed_at: '2026-06-12T18:00:00Z',
    items: [
      {
        id: 9004,
        order_id: 1003,
        sku_id: 121,
        qty: 1,
        unit_price_minor: 76000,
        unit_price_cny: 580000,
        title_snapshot: { enUS: 'Comma Cups (set of 4)', zhCN: '逗号杯（四只）' },
        figure_seed: 112,
        figure_kind: 'teapot' as const,
      },
      {
        id: 9005,
        order_id: 1003,
        sku_id: 111,
        qty: 1,
        unit_price_minor: 246200,
        unit_price_cny: 1880000,
        title_snapshot: { enUS: 'Imperial Yellow Ginger Jar', zhCN: '明黄地瓜楞罐' },
        figure_seed: 111,
        figure_kind: 'jar' as const,
      },
    ],
  },
]

export const SEED_WISHLIST = [
  { user_id: 'u_9f3a', sku_id: 61, added_at: '2026-07-19T10:00:00Z' },
  { user_id: 'u_9f3a', sku_id: 41, added_at: '2026-08-01T10:00:00Z' },
]

export const SEED_ITINERARIES = [
  {
    id: 5001,
    user_id: 'u_9f3a',
    status: 'pending' as const,
    arrival_date: '2026-10-12',
    duration_days: 5,
    flexible: true,
    adults: 2,
    children: 0,
    interests: ['pottery-workshop', 'kiln-sites', 'artist-studios', 'local-food'],
    budget: { currency: 'USD' as const, min_minor: 150000, max_minor: 300000 },
    pace: 'balanced' as const,
    services: {
      guide: 'english' as const,
      hotel: true,
      hotel_level: 'comfort' as const,
      pickup: true,
      experience: true,
    },
    contact: {
      channel: 'email' as const,
      notes: 'Honeymoon trip — surprise pottery class would be lovely.',
    },
    locale: 'en-US',
    sla_deadline: '2026-08-15T09:00:00Z',
    submitted_at: '2026-08-14T09:00:00Z',
  },
  {
    id: 5002,
    user_id: 'u_9f3a',
    status: 'confirmed' as const,
    arrival_date: '2026-04-06',
    duration_days: 3,
    flexible: false,
    adults: 2,
    children: 1,
    interests: ['museums', 'ceramic-shopping', 'photography'],
    budget: { currency: 'USD' as const, min_minor: 100000, max_minor: 200000 },
    pace: 'relaxed' as const,
    services: {
      guide: 'none' as const,
      hotel: true,
      hotel_level: 'budget' as const,
      pickup: false,
      experience: false,
    },
    contact: { channel: 'email' as const, notes: '' },
    locale: 'en-US',
    sla_deadline: '2026-01-20T08:00:00Z',
    submitted_at: '2026-01-19T08:00:00Z',
  },
]

/* ------------------------------------------------------------------ */
/* Certificates (one per product; codes deterministic)                 */
/* ------------------------------------------------------------------ */

export const CERTIFICATES = [
  {
    id: 1,
    product_id: 4,
    cert_code: 'JDZ-2026-A7F3',
    issued_at: '2026-07-10T08:00:00Z',
    provenance: [
      {
        id: 1,
        kind: 'created' as const,
        detail: 'Thrown and painted at Yu Studio, Jingdezhen',
        at: '2026-07-02T08:00:00Z',
      },
    ],
  },
  {
    id: 2,
    product_id: 6,
    cert_code: 'JDZ-2026-B2C8',
    issued_at: '2026-02-25T08:00:00Z',
    provenance: [
      {
        id: 1,
        kind: 'created' as const,
        detail: 'Wood-fired at Sanbao valley kiln, 72h firing',
        at: '2026-02-18T08:00:00Z',
      },
      {
        id: 2,
        kind: 'sold' as const,
        detail: 'Sold via Jingdezhen Ceramics Platform · Order #1003',
        at: '2026-05-30T11:05:00Z',
      },
    ],
  },
  {
    id: 3,
    product_id: 9,
    cert_code: 'JDZ-2026-D9E1',
    issued_at: '2026-07-03T08:00:00Z',
    provenance: [
      {
        id: 1,
        kind: 'created' as const,
        detail: 'Painted at Taoxichuan studio',
        at: '2026-06-28T08:00:00Z',
      },
    ],
  },
  {
    id: 4,
    product_id: 1,
    cert_code: 'JDZ-2025-C4A6',
    issued_at: '2026-05-14T08:00:00Z',
    provenance: [
      {
        id: 1,
        kind: 'created' as const,
        detail: 'Painted by Chen Yuqing at Xianghu Kiln',
        at: '2026-05-10T08:00:00Z',
      },
    ],
  },
]

/* ------------------------------------------------------------------ */
/* Itinerary interest options (CMS-editable, PRD §3.3.2 launch list)   */
/* ------------------------------------------------------------------ */

export const INTEREST_OPTIONS: Array<{ key: string; translations: Translation<string> }> = [
  {
    key: 'pottery-workshop',
    translations: { enUS: 'Pottery-making workshop', zhCN: '制陶体验课' },
  },
  { key: 'kiln-sites', translations: { enUS: 'Kiln & heritage sites', zhCN: '窑址与遗迹' } },
  {
    key: 'artist-studios',
    translations: { enUS: 'Artist studio visits', zhCN: '艺术家工作室拜访' },
  },
  { key: 'ceramic-shopping', translations: { enUS: 'Ceramic shopping', zhCN: '瓷器购物' } },
  { key: 'museums', translations: { enUS: 'Museums', zhCN: '博物馆' } },
  { key: 'local-food', translations: { enUS: 'Local food', zhCN: '在地美食' } },
  { key: 'photography', translations: { enUS: 'Photography', zhCN: '摄影' } },
  {
    key: 'countryside-sanbao',
    translations: { enUS: 'Countryside (Sanbao)', zhCN: '乡野（三宝）' },
  },
]

/* ------------------------------------------------------------------ */
/* FX rates + shipping tiers (mock "server-side" pricing inputs)       */
/* ------------------------------------------------------------------ */

/** CNY per 1 unit of currency, after the default 2% markup (TDD §7). */
export const FX_RATES = {
  USD: { rate_to_cny: 7.16, fetched_at: '2026-08-14T16:05:00Z' },
  EUR: { rate_to_cny: 7.78, fetched_at: '2026-08-14T16:05:00Z' },
  GBP: { rate_to_cny: 9.05, fetched_at: '2026-08-14T16:05:00Z' },
}

export const FX_MARKUP = 0.02

/** Per-country weight tiers (fee in CNY minor units). PRD §3.2.3. */
export const SHIPPING_TIERS: Array<{
  country: string
  max_weight_grams: number
  fee_cny: number
}> = [
  { country: 'US', max_weight_grams: 1000, fee_cny: 8800 },
  { country: 'US', max_weight_grams: 3000, fee_cny: 16800 },
  { country: 'US', max_weight_grams: 5000, fee_cny: 24800 },
  { country: 'US', max_weight_grams: 15000, fee_cny: 38800 },
  { country: 'GB', max_weight_grams: 1000, fee_cny: 9800 },
  { country: 'GB', max_weight_grams: 3000, fee_cny: 17800 },
  { country: 'GB', max_weight_grams: 5000, fee_cny: 26800 },
  { country: 'GB', max_weight_grams: 10000, fee_cny: 42800 },
  { country: 'DE', max_weight_grams: 1000, fee_cny: 9600 },
  { country: 'DE', max_weight_grams: 3000, fee_cny: 17600 },
  { country: 'DE', max_weight_grams: 5000, fee_cny: 26200 },
  { country: 'DE', max_weight_grams: 10000, fee_cny: 41800 },
  { country: 'FR', max_weight_grams: 1000, fee_cny: 9900 },
  { country: 'FR', max_weight_grams: 3000, fee_cny: 17900 },
  { country: 'FR', max_weight_grams: 5000, fee_cny: 27000 },
  { country: 'FR', max_weight_grams: 10000, fee_cny: 43200 },
  { country: 'CA', max_weight_grams: 1000, fee_cny: 9200 },
  { country: 'CA', max_weight_grams: 3000, fee_cny: 17200 },
  { country: 'CA', max_weight_grams: 5000, fee_cny: 25800 },
  { country: 'CA', max_weight_grams: 10000, fee_cny: 40800 },
  { country: 'AU', max_weight_grams: 1000, fee_cny: 10800 },
  { country: 'AU', max_weight_grams: 3000, fee_cny: 19800 },
  { country: 'AU', max_weight_grams: 5000, fee_cny: 29800 },
  { country: 'JP', max_weight_grams: 1000, fee_cny: 7800 },
  { country: 'JP', max_weight_grams: 3000, fee_cny: 13800 },
  { country: 'JP', max_weight_grams: 5000, fee_cny: 20800 },
  { country: 'SG', max_weight_grams: 1000, fee_cny: 6800 },
  { country: 'SG', max_weight_grams: 3000, fee_cny: 11800 },
  { country: 'SG', max_weight_grams: 5000, fee_cny: 17800 },
]

/** Countries offered in the address form (mock subset). */
export const SHIPPABLE_COUNTRIES = ['US', 'GB', 'DE', 'FR', 'NL', 'CA', 'AU', 'JP', 'SG']

export const CONTACT = { email: 'hello@jdz-atelier.example', phone: '+86 798 8555 0100' }
