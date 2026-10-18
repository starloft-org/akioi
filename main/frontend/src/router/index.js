import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/store/user'

const UserLayout = () => import('@/layouts/UserLayout.vue')
const AdminLayout = () => import('@/layouts/AdminLayout.vue')

const routes = [
  {
    path: '/',
    component: UserLayout,
    children: [
      { path: '', name: 'home', component: () => import('@/views/Home.vue') },
      { path: 'problems', name: 'problems', component: () => import('@/views/ProblemList.vue') },
      { path: 'problems/:id', name: 'problem-detail', component: () => import('@/views/ProblemDetail.vue') },
      { path: 'problems/:id/solutions', name: 'solutions', component: () => import('@/views/SolutionList.vue') },
      { path: 'solutions/:id', name: 'solution-detail', component: () => import('@/views/SolutionDetail.vue') },
      { path: 'ranking', name: 'ranking', component: () => import('@/views/Ranking.vue') },
      { path: 'discussions', name: 'discussions', component: () => import('@/views/DiscussionList.vue') },
      { path: 'discussions/:id', name: 'discussion-detail', component: () => import('@/views/DiscussionDetail.vue') },
      { path: 'submissions', name: 'submissions', component: () => import('@/views/SubmissionList.vue') },
      { path: 'submissions/:id', name: 'submission-detail', component: () => import('@/views/SubmissionDetail.vue') },
      { path: 'contests', name: 'contests', component: () => import('@/views/ContestList.vue') },
      { path: 'contests/:id', name: 'contest-detail', component: () => import('@/views/ContestDetail.vue') },
      { path: 'contests/:id/rank', name: 'contest-rank', component: () => import('@/views/ContestRank.vue') },
      {
        path: 'contests/:id/submissions',
        name: 'contest-submissions',
        component: () => import('@/views/ContestSubmissions.vue')
      },
      { path: 'users/:id', name: 'user-profile', component: () => import('@/views/UserProfile.vue') },
      {
        path: 'profile',
        name: 'profile',
        meta: { requiresAuth: true },
        component: () => import('@/views/Profile.vue')
      }
    ]
  },
  { path: '/login', name: 'login', component: () => import('@/views/Login.vue') },
  { path: '/auth/wechat', name: 'wechat-callback', component: () => import('@/views/WechatCallback.vue') },
  { path: '/register', name: 'register', component: () => import('@/views/Register.vue') },
  { path: '/forgot-password', name: 'forgot-password', component: () => import('@/views/ForgotPassword.vue') },
  {
    path: '/admin',
    component: AdminLayout,
    meta: { requiresAuth: true, requiresAdmin: true },
    children: [
      { path: '', name: 'admin-dashboard', component: () => import('@/views/admin/Dashboard.vue') },
      { path: 'users', name: 'admin-users', component: () => import('@/views/admin/Users.vue') },
      { path: 'problems', name: 'admin-problems', component: () => import('@/views/admin/Problems.vue') },
      {
        path: 'problems/new',
        name: 'admin-problem-new',
        component: () => import('@/views/admin/ProblemEdit.vue')
      },
      {
        path: 'problems/:id/edit',
        name: 'admin-problem-edit',
        component: () => import('@/views/admin/ProblemEdit.vue')
      },
      {
        path: 'problems/:id/testdata',
        name: 'admin-testdata',
        component: () => import('@/views/admin/Testdata.vue')
      },
      { path: 'contests', name: 'admin-contests', component: () => import('@/views/admin/Contests.vue') },
      {
        path: 'contests/new',
        name: 'admin-contest-new',
        component: () => import('@/views/admin/ContestEdit.vue')
      },
      {
        path: 'contests/:id/edit',
        name: 'admin-contest-edit',
        component: () => import('@/views/admin/ContestEdit.vue')
      },
      {
        path: 'submissions',
        name: 'admin-submissions',
        component: () => import('@/views/admin/Submissions.vue')
      },
      {
        path: 'announcements',
        name: 'admin-announcements',
        component: () => import('@/views/admin/Announcements.vue')
      },
      { path: 'settings', name: 'admin-settings', component: () => import('@/views/admin/Settings.vue') },
      { path: 'logs', name: 'admin-logs', component: () => import('@/views/admin/Logs.vue') }
    ]
  },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/views/NotFound.vue') }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 })
})

router.beforeEach(async (to) => {
  const store = useUserStore()
  if (!store.site.app_name) {
    await store.loadSite()
  }
  const needAuth = to.matched.some((r) => r.meta.requiresAuth)
  const needAdmin = to.matched.some((r) => r.meta.requiresAdmin)

  if (!store.user && store.token) {
    await store.loadMe()
  }
  if (needAuth && !store.user) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (needAdmin && !store.isAdmin) {
    return { name: 'home' }
  }
  return true
})

export default router
