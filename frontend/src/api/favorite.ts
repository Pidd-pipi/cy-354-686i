import request from '../utils/request'
import type { FavoriteProduct, PriceAlert, Product } from '../types'

export function addFavorite(productId: number) {
  return request.post<never, { code: number; message: string; data: { id: number; product_id: number } }>(
    `/products/${productId}/favorite`,
  )
}

export function removeFavorite(productId: number) {
  return request.delete<never, { code: number; message: string; data: { product_id: number; favorited: boolean } }>(
    `/products/${productId}/favorite`,
  )
}

export function listMyFavorites() {
  return request.get<never, { code: number; message: string; data: { items: FavoriteProduct[] } }>(
    '/users/me/favorites',
  )
}

export function listMyFavoriteIds() {
  return request.get<never, { code: number; message: string; data: { product_ids: number[] } }>(
    '/users/me/favorites/ids',
  )
}

export function listMyPriceAlerts() {
  return request.get<never, { code: number; message: string; data: { items: PriceAlert[] } }>(
    '/users/me/price-alerts',
  )
}

export function markPriceAlertRead(id: number) {
  return request.post<never, { code: number; message: string; data: { id: number; read: boolean } }>(
    `/users/me/price-alerts/${id}/read`,
  )
}

export function markAllPriceAlertsRead() {
  return request.post<never, { code: number; message: string; data: { read: boolean } }>(
    '/users/me/price-alerts/read-all',
  )
}

export function updateProductPrice(productId: number, price: number) {
  return request.put<never, { code: number; message: string; data: Product }>(
    `/products/${productId}/price`,
    { price },
  )
}

export function listMyProducts() {
  return request.get<never, { code: number; message: string; data: { items: Product[] } }>(
    '/users/me/products',
  )
}
