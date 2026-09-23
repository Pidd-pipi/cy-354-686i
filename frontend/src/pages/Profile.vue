<template>
  <div class="page">
    <h2>个人中心</h2>
    <template v-if="authStore.user">
      <el-card class="profile-card">
        <div class="profile-head">
          <el-avatar :size="64" :src="authStore.user.avatar || ''">{{ authStore.user.nickname.slice(0, 1) }}</el-avatar>
          <div class="profile-info">
            <h3>{{ authStore.user.nickname }}</h3>
            <p>{{ authStore.user.phone }} · {{ roleLabel(authStore.user.role) }} · {{ authStore.user.campus }}</p>
          </div>
          <div class="credit-box">
            <div class="credit-label">信誉分</div>
            <div class="credit-value">{{ authStore.user.credit_score }}</div>
            <div class="credit-level">{{ creditLevel(authStore.user.credit_score) }}</div>
          </div>
        </div>
      </el-card>

      <el-card class="section">
        <template #header>
          <div class="section-head">
            🔔 降价提醒
            <el-badge v-if="unreadAlertCount > 0" :value="unreadAlertCount" class="alert-badge" />
          </div>
        </template>
        <el-table :data="alerts" empty-text="暂无降价提醒">
          <el-table-column label="商品" min-width="180">
            <template #default="{ row }">
              <el-link type="primary" @click="goProduct(row.product_id)">{{ row.product_title }}</el-link>
              <el-tag v-if="!row.purchasable" size="small" type="info" class="off-tag">不可购买</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="原价" width="100">
            <template #default="{ row }"><span class="old-price">¥{{ row.old_price.toFixed(2) }}</span></template>
          </el-table-column>
          <el-table-column label="新价" width="100">
            <template #default="{ row }"><span class="new-price">¥{{ row.new_price.toFixed(2) }}</span></template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag v-if="!row.is_read" size="small" type="danger">未读</el-tag>
              <el-tag v-else size="small" type="info">已读</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时间" width="170">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="150">
            <template #default="{ row }">
              <el-button v-if="!row.is_read" size="small" link type="primary" @click="markAlertRead(row.id)">标为已读</el-button>
              <el-button size="small" link type="danger" @click="removeAlertRow(row.id)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-card class="section">
        <template #header>⭐ 我的收藏</template>
        <el-row :gutter="16">
          <el-col v-for="f in favorites" :key="f.id" :span="8" class="fav-col">
            <el-card v-if="f.product" shadow="never" class="fav-card">
              <div class="fav-head">
                <el-link type="primary" :underline="false" @click="goProduct(f.product_id)">{{ f.product.title }}</el-link>
                <el-tag v-if="!f.purchasable" size="small" type="info">不可购买</el-tag>
              </div>
              <div class="fav-price">
                <span class="new-price">¥{{ f.product.price.toFixed(2) }}</span>
                <el-tag :type="productStatusType(f.product.status) as any" size="small">{{ productStatusLabel(f.product.status) }}</el-tag>
              </div>
              <div class="fav-meta">{{ categoryLabel(f.product.category) }} · {{ f.product.campus }} · ❤️ {{ f.product.favorite_count ?? 0 }}</div>
              <div class="fav-actions">
                <el-button size="small" type="primary" :disabled="!f.purchasable" @click="buy(f.product!)">
                  {{ f.purchasable ? '购买' : '不可购买' }}
                </el-button>
                <el-button size="small" @click="chat(f.product!)">私信</el-button>
                <el-button size="small" type="danger" plain @click="unfavorite(f.product_id)">取消收藏</el-button>
              </div>
            </el-card>
          </el-col>
        </el-row>
        <el-empty v-if="favorites.length === 0" description="还没有收藏，去商品广场挑挑吧" />
      </el-card>

      <el-card class="section">
        <template #header>📦 我发布的</template>
        <el-table :data="myProducts" empty-text="还没有发布商品">
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column label="商品" min-width="160">
            <template #default="{ row }">
              <el-link type="primary" @click="goProduct(row.id)">{{ row.title }}</el-link>
            </template>
          </el-table-column>
          <el-table-column label="价格" width="120">
            <template #default="{ row }">¥{{ row.price.toFixed(2) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="productStatusType(row.status) as any" size="small">{{ productStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="收藏" width="80">
            <template #default="{ row }">❤️ {{ row.favorite_count ?? 0 }}</template>
          </el-table-column>
          <el-table-column label="操作" width="200">
            <template #default="{ row }">
              <el-button v-if="row.status === 'on_sale'" size="small" type="warning" link @click="openReprice(row)">调价</el-button>
              <el-button v-if="row.status === 'on_sale'" size="small" type="danger" link @click="takeDown(row.id)">下架</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-card class="section">
        <template #header>⭐ 收到的评价</template>
        <el-table :data="reviews">
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column label="评价">
            <template #default="{ row }">
              <el-tag size="small">{{ ratingLabel(row.rating) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="content" label="内容" />
          <el-table-column label="时间">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </el-card>
    </template>
    <el-empty v-else description="请先登录">
      <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
    </el-empty>

    <el-dialog v-model="repriceVisible" title="调整价格（降价会通知收藏者）" width="380px">
      <p class="reprice-title">{{ repriceForm.title }}</p>
      <p class="reprice-current">当前价格：¥{{ repriceForm.oldPrice.toFixed(2) }}</p>
      <el-input-number v-model="repriceForm.price" :min="0.01" :precision="2" />
      <el-alert
        v-if="repriceForm.price < repriceForm.oldPrice"
        title="新价低于原价，保存后将通知所有收藏该商品的学生"
        type="success"
        :closable="false"
        class="reprice-hint"
      />
      <template #footer>
        <el-button @click="repriceVisible = false">取消</el-button>
        <el-button type="primary" :loading="repriceSaving" @click="submitReprice">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/authStore'
import { roleLabel } from '../constants/user'
import { ratingLabel } from '../constants/trade'
import { categoryLabel, productStatusLabel, productStatusType } from '../constants/product'
import { listMyReviews } from '../api/review'
import { listProducts, removeProduct, updateProductPrice } from '../api/product'
import {
  listMyFavorites,
  removeFavorite,
} from '../api/favorite'
import { createTradeOrder } from '../api/tradeOrder'
import { createConversation } from '../api/conversation'
import { usePriceAlertStore } from '../stores/priceAlertStore'
import { formatDateTime } from '../utils/dateFormat'
import type { FavoriteItem, PriceAlert, Product, Review } from '../types'

const authStore = useAuthStore()
const router = useRouter()
const alertStore = usePriceAlertStore()
const reviews = ref<Review[]>([])
const favorites = ref<FavoriteItem[]>([])
const alerts = ref<PriceAlert[]>([])
const myProducts = ref<Product[]>([])

const unreadAlertCount = computed(() => alerts.value.filter((a) => !a.is_read).length)

const repriceVisible = ref(false)
const repriceSaving = ref(false)
const repriceForm = reactive({ id: 0, title: '', oldPrice: 0, price: 0 })

function goProduct(productId: number) {
  router.push({ path: '/products', query: { highlight: productId } })
}

async function loadProfileData() {
  const [favRes, reviewRes, productRes] = await Promise.all([
    listMyFavorites(),
    listMyReviews(),
    listProducts({ seller_id: authStore.user!.id, page: 1, page_size: 100 }),
  ])
  await alertStore.fetchAlerts(true)
  favorites.value = favRes.data
  alerts.value = [...alertStore.alerts]
  reviews.value = reviewRes.data
  myProducts.value = productRes.data.items
}

async function buy(p: Product) {
  await createTradeOrder(p.id)
  ElMessage.success('已下单，等待卖家确认')
}

async function chat(p: Product) {
  await createConversation(p.id)
  ElMessage.success('已发起私信')
  router.push('/messages')
}

async function unfavorite(productId: number) {
  await removeFavorite(productId)
  favorites.value = favorites.value.filter((f) => f.product_id !== productId)
  ElMessage.success('已取消收藏')
}

async function markAlertRead(id: number) {
  await alertStore.markRead(id)
  alerts.value = [...alertStore.alerts]
}

async function removeAlertRow(id: number) {
  await alertStore.remove(id)
  alerts.value = [...alertStore.alerts]
  ElMessage.success('提醒已删除')
}

function openReprice(p: Product) {
  repriceForm.id = p.id
  repriceForm.title = p.title
  repriceForm.oldPrice = p.price
  repriceForm.price = p.price
  repriceVisible.value = true
}

async function submitReprice() {
  if (repriceForm.price <= 0) {
    ElMessage.warning('价格必须大于0')
    return
  }
  repriceSaving.value = true
  try {
    const res = await updateProductPrice(repriceForm.id, repriceForm.price)
    const target = myProducts.value.find((p) => p.id === repriceForm.id)
    if (target) target.price = res.data.price
    repriceVisible.value = false
    ElMessage.success(
      repriceForm.price < repriceForm.oldPrice ? '价格已更新，已通知收藏者' : '价格已更新',
    )
  } finally {
    repriceSaving.value = false
  }
}

async function takeDown(id: number) {
  await ElMessageBox.confirm('确定下架该商品吗？下架后收藏者将无法购买，收藏仍会保留。', '提示', {
    type: 'warning',
  })
  await removeProduct(id)
  const p = myProducts.value.find((x) => x.id === id)
  if (p) p.status = 'removed'
  ElMessage.success('已下架')
}

function creditLevel(score: number): string {
  if (score >= 200) return '极佳'
  if (score >= 150) return '优秀'
  if (score >= 100) return '良好'
  if (score >= 60) return '一般'
  return '待提升'
}

onMounted(async () => {
  if (!authStore.token) return
  await loadProfileData()
})
</script>

<style scoped>
.profile-card {
  margin-bottom: 16px;
}
.profile-head {
  display: flex;
  align-items: center;
  gap: 16px;
}
.profile-info h3 {
  margin: 0;
}
.profile-info p {
  margin: 4px 0 0;
  color: #909399;
  font-size: 13px;
}
.credit-box {
  margin-left: auto;
  text-align: center;
}
.credit-label {
  color: #909399;
  font-size: 12px;
}
.credit-value {
  font-size: 28px;
  font-weight: 700;
  color: #e6a23c;
}
.credit-level {
  font-size: 12px;
  color: #909399;
}
.section {
  margin-bottom: 16px;
}
.section-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.alert-badge {
  margin-left: 4px;
}
.old-price {
  color: #909399;
  text-decoration: line-through;
}
.new-price {
  color: #f56c6c;
  font-weight: 700;
}
.off-tag {
  margin-left: 6px;
}
.fav-col {
  margin-bottom: 16px;
}
.fav-card {
  border: 1px solid #ebeef5;
}
.fav-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.fav-price {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 8px 0;
}
.fav-meta {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}
.fav-actions {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.reprice-title {
  font-weight: 600;
  margin: 0 0 4px;
}
.reprice-current {
  color: #909399;
  font-size: 13px;
  margin: 0 0 12px;
}
.reprice-hint {
  margin-top: 12px;
}
</style>
