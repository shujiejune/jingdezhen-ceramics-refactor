import { createFileRoute, Link } from '@tanstack/react-router'
import {
  ArrowLeft,
  CheckCircle,
  XCircle,
  Eye,
  PencilSimple,
  Plus,
  Trash,
} from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { StatusBadge } from '~/components/admin/ContentTable'
import { Badge, Button, FieldError, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { ContentStatus, Product, SKU, SKUAttributes } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/products/$id')({
  component: ProductDetailPage,
})

const ATTR_KEYS: (keyof SKUAttributes)[] = [
  'size',
  'technique',
  'glaze',
  'edition_type',
  'edition_number',
  'year',
  'kiln',
]

function ProductDetailPage() {
  const { id } = Route.useParams()
  const { t, locale } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [item, setItem] = useState<Product | null | undefined>(undefined)
  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [metaTitle, setMetaTitle] = useState('')
  const [metaDescription, setMetaDescription] = useState('')
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [attributes, setAttributes] = useState<Record<string, string>>({})
  const [skus, setSkus] = useState<SKU[]>([])
  const [newSku, setNewSku] = useState({
    sku_code: '',
    price_cny: '',
    stock: '',
    weight_grams: '',
    low_stock_threshold: '5',
  })

  useEffect(() => {
    if (!ready || !token) return
    api
      .adminGetProduct(token, id)
      .then((s) => {
        setItem(s)
        setTitle(s.title ?? '')
        setSlug(s.slug ?? '')
        setDescription(s.description ?? '')
        setMetaTitle(s.meta_title ?? '')
        setMetaDescription(s.meta_description ?? '')
        setSkus(s.skus ?? [])
      })
      .catch(() => setItem(null))
  }, [ready, token, id])

  const canWrite = hasPermission('product.write')
  const canPublish = hasPermission('product.publish')

  const save = async () => {
    if (!token) return
    setSaving(true)
    setErr(null)
    try {
      const updated = await api.adminUpdateProduct(token, Number(id), {
        title,
        slug,
        description,
        meta_title: metaTitle,
        meta_description: metaDescription,
        attributes,
      })
      setItem(updated)
      push({ title: t('admin.common.saved'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setSaving(false)
    }
  }

  const workflow = async (action: 'submit' | 'approve' | 'reject' | 'unpublish') => {
    if (!token) return
    try {
      const fn = {
        submit: api.adminSubmitProduct,
        approve: api.adminApproveProduct,
        reject: api.adminRejectProduct,
        unpublish: api.adminUnpublishProduct,
      }[action]
      const updated = await fn(token, Number(id))
      setItem(updated)
      const msg = {
        submit: 'admin.workflow.submitted',
        approve: 'admin.workflow.approved',
        reject: 'admin.workflow.rejected',
        unpublish: 'admin.workflow.unpublished',
      }[action]
      push({ title: t(msg as Parameters<typeof t>[0]), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const addSku = async () => {
    if (!token) return
    const code = newSku.sku_code.trim()
    if (!code) return
    try {
      const created = await api.adminCreateSKU(token, Number(id), {
        sku_code: code,
        price_cny: Number(newSku.price_cny) || 0,
        stock: Number(newSku.stock) || 0,
        weight_grams: Number(newSku.weight_grams) || 0,
        low_stock_threshold: Number(newSku.low_stock_threshold) || 5,
        attributes: {},
        is_active: true,
      })
      setSkus((prev) => [...prev, created])
      setNewSku({
        sku_code: '',
        price_cny: '',
        stock: '',
        weight_grams: '',
        low_stock_threshold: '5',
      })
      push({ title: t('admin.sku.add'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const deleteSku = async (skuId: number) => {
    if (!token) return
    try {
      await api.adminDeleteSKU(token, skuId)
      setSkus((prev) => prev.filter((s) => s.id !== skuId))
      push({ title: t('admin.sku.delete'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  if (item === undefined) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-6 w-6 text-cobalt-400" />
      </div>
    )
  }
  if (item === null) {
    return (
      <div className="py-32 text-center text-[0.88rem] text-ink-400">
        <p>{t('admin.common.empty')}</p>
        <Link to={`/${locale}/admin/products` as never} className="mt-4 link-quiet">
          {t('admin.common.back')}
        </Link>
      </div>
    )
  }

  const status = item.status as ContentStatus

  return (
    <div>
      <Link
        to={`/${locale}/admin/products` as never}
        className="inline-flex items-center gap-1.5 text-[0.84rem] text-ink-500 hover:text-cobalt-700"
      >
        <ArrowLeft size={14} /> {t('admin.content.products')}
      </Link>

      <div className="mt-4 flex items-center gap-3">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{item.title}</h2>
        <StatusBadge status={status} />
      </div>

      {err && <FieldError>{err}</FieldError>}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <div className="card-surface p-6">
          <div className="flex flex-col gap-3">
            <div>
              <label className="label-base">{t('admin.common.title')}</label>
              <input
                className="input-base"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">{t('admin.common.slug')}</label>
              <input
                className="input-base"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">Description</label>
              <textarea
                className="input-base min-h-24"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">Meta title</label>
              <input
                className="input-base"
                value={metaTitle}
                onChange={(e) => setMetaTitle(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">Meta description</label>
              <textarea
                className="input-base min-h-16"
                value={metaDescription}
                onChange={(e) => setMetaDescription(e.target.value)}
                disabled={!canWrite}
              />
            </div>
          </div>
        </div>

        <div className="card-surface p-6">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-[0.88rem] font-semibold text-ink-700">
              {t('admin.common.status')}
            </h3>
            <Badge tone="neutral">{status}</Badge>
          </div>
          <div className="flex flex-col gap-2.5">
            {canWrite && status === 'draft' && (
              <Button variant="secondary" onClick={() => void workflow('submit')}>
                <CheckCircle size={15} /> {t('admin.workflow.submit')}
              </Button>
            )}
            {canPublish && status === 'in_review' && (
              <>
                <Button variant="secondary" onClick={() => void workflow('approve')}>
                  <CheckCircle size={15} /> {t('admin.workflow.approve')}
                </Button>
                <Button variant="danger" onClick={() => void workflow('reject')}>
                  <XCircle size={15} /> {t('admin.workflow.reject')}
                </Button>
              </>
            )}
            {canPublish && status === 'published' && (
              <Button variant="secondary" onClick={() => void workflow('unpublish')}>
                <Eye size={15} /> {t('admin.workflow.unpublish')}
              </Button>
            )}
            {canWrite && (
              <Button onClick={() => void save()} loading={saving}>
                <PencilSimple size={15} /> {t('admin.common.save')}
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Attributes JSONB editor */}
      {canWrite && (
        <div className="mt-6 card-surface p-6">
          <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">Attributes</h3>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {ATTR_KEYS.map((key) => (
              <div key={key}>
                <label className="label-base">{key}</label>
                <input
                  className="input-base"
                  value={attributes[key as string] ?? ''}
                  onChange={(e) => setAttributes((prev) => ({ ...prev, [key]: e.target.value }))}
                />
              </div>
            ))}
          </div>
        </div>
      )}

      {/* SKU management */}
      <div className="mt-6 card-surface p-6">
        <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">SKUs</h3>

        {skus.length > 0 ? (
          <div className="overflow-x-auto rounded-lg border border-cobalt-50">
            <table className="w-full text-left text-[0.82rem]">
              <thead className="bg-wash/50">
                <tr className="border-b border-cobalt-50">
                  <th className="px-3 py-2 font-semibold text-ink-500">{t('admin.sku.skuCode')}</th>
                  <th className="px-3 py-2 font-semibold text-ink-500">
                    {t('admin.sku.priceCny')}
                  </th>
                  <th className="px-3 py-2 font-semibold text-ink-500">{t('admin.sku.stock')}</th>
                  <th className="px-3 py-2 font-semibold text-ink-500">{t('admin.sku.weight')}</th>
                  <th className="px-3 py-2 font-semibold text-ink-500">
                    {t('admin.sku.lowStock')}
                  </th>
                  {canWrite && <th className="px-3 py-2" />}
                </tr>
              </thead>
              <tbody>
                {skus.map((sku) => (
                  <tr key={sku.id} className="border-b border-cobalt-50/60">
                    <td className="px-3 py-2 text-ink-700">{sku.sku_code}</td>
                    <td className="px-3 py-2 text-ink-700">{sku.price_cny}</td>
                    <td className="px-3 py-2 text-ink-700">{sku.stock}</td>
                    <td className="px-3 py-2 text-ink-700">{sku.weight_grams}</td>
                    <td className="px-3 py-2 text-ink-700">{sku.low_stock_threshold}</td>
                    {canWrite && (
                      <td className="px-3 py-2">
                        <button
                          type="button"
                          onClick={() => void deleteSku(sku.id)}
                          className="text-ink-400 transition hover:text-[color:var(--color-danger)]"
                        >
                          <Trash size={14} />
                        </button>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="py-4 text-center text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
        )}

        {canWrite && (
          <div className="mt-4 flex flex-wrap items-end gap-3 rounded-lg border border-dashed border-cobalt-100 p-4">
            <div>
              <label className="label-base">{t('admin.sku.skuCode')}</label>
              <input
                className="input-base"
                value={newSku.sku_code}
                onChange={(e) => setNewSku((s) => ({ ...s, sku_code: e.target.value }))}
              />
            </div>
            <div>
              <label className="label-base">{t('admin.sku.priceCny')}</label>
              <input
                className="input-base"
                type="number"
                value={newSku.price_cny}
                onChange={(e) => setNewSku((s) => ({ ...s, price_cny: e.target.value }))}
              />
            </div>
            <div>
              <label className="label-base">{t('admin.sku.stock')}</label>
              <input
                className="input-base"
                type="number"
                value={newSku.stock}
                onChange={(e) => setNewSku((s) => ({ ...s, stock: e.target.value }))}
              />
            </div>
            <div>
              <label className="label-base">{t('admin.sku.weight')}</label>
              <input
                className="input-base"
                type="number"
                value={newSku.weight_grams}
                onChange={(e) => setNewSku((s) => ({ ...s, weight_grams: e.target.value }))}
              />
            </div>
            <div>
              <label className="label-base">{t('admin.sku.lowStock')}</label>
              <input
                className="input-base"
                type="number"
                value={newSku.low_stock_threshold}
                onChange={(e) => setNewSku((s) => ({ ...s, low_stock_threshold: e.target.value }))}
              />
            </div>
            <Button variant="secondary" onClick={() => void addSku()}>
              <Plus size={15} /> {t('admin.sku.add')}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
