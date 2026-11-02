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
      { path: 'problems/:id', name: 'problem-detail', component: () => import('@/views/ProblemDetail.vue'), meta: { wide: true } },
      { path: 'problems/:id/solutions', name: 'solutions', component: () => import('@/views/SolutionList.vue') },
      { path: 'solutions/:id', name: 'solution-detail', component: () => import('@/views/SolutionDetail.vue') },
      { path: 'ranking', name: 'ranking', component: () => import('@/views/Ranking.vue') },
      { path: 'discussions', name: 'discussions', component: () => import('@/views/DiscussionList.vue') },
      { path: 'discussions/:id', name: 'discussion-detail', component: () => import('@/views/DiscussionDetail.vue') },
      { path: 'submissions', name: 'submissions', component: () => import('@/views/SubmissionList.vue') },
      { path: 'submissions/:id', name: 'submission-detail', component: () => import('@/views/SubmissionDetail.vue') },
      { path: 'teams', name: 'teams', component: () => import('@/views/TeamList.vue') },
      { path: 'teams/:id', name: 'team-detail', component: () => import('@/views/TeamDetail.vue') },
      {
        path: 'my/problems',
        name: 'my-problems',
        meta: { requiresAuth: true },
        component: () => import('@/views/MyProblems.vue')
      },
      {
        path: 'problem-sets',
        name: 'problem-sets',
        meta: { requiresAuth: true },
        component: () => import('@/views/ProblemSets.vue')
      },
      {
        path: 'problem-sets/personal/:id',
        name: 'problem-set-personal',
        meta: { requiresAuth: true },
        component: () => import('@/views/ProblemSetDetail.vue')
      },
      {
        path: 'problem-sets/team/:teamId/:setId',
        name: 'problem-set-team',
        meta: { requiresAuth: true },
        component: () => import('@/views/ProblemSetDetail.vue')
      },
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
      {
        path: 'users',
        name: 'admin-users',
        meta: { permission: 'users' },
        component: () => import('@/views/admin/Users.vue')
      },
      {
        path: 'admins',
        name: 'admin-admins',
        meta: { superAdmin: true },
        component: () => import('@/views/admin/Admins.vue')
      },
      {
        path: 'problems',
        name: 'admin-problems',
        meta: { permission: 'problems' },
        component: () => import('@/views/admin/Problems.vue')
      },
      {
        path: 'solutions',
        name: 'admin-solutions',
        meta: { permission: 'solutions' },
        component: () => import('@/views/admin/Solutions.vue')
      },
      {
        path: 'problem-banks',
        name: 'admin-problem-banks',
        meta: { permission: 'problem_banks' },
        component: () => import('@/views/admin/ProblemBanks.vue')
      },
      {
        path: 'problems/new',
        name: 'admin-problem-new',
        meta: { permission: 'problems' },
        component: () => import('@/views/admin/ProblemEdit.vue')
      },
      {
        path: 'problems/:id/edit',
        name: 'admin-problem-edit',
        meta: { permission: 'problems' },
        component: () => import('@/views/admin/ProblemEdit.vue')
      },
      {
        path: 'problems/:id/testdata',
        name: 'admin-testdata',
        meta: { permission: 'problems' },
        component: () => import('@/views/admin/Testdata.vue')
      },
      {
        path: 'contests',
        name: 'admin-contests',
        meta: { permission: 'contests' },
        component: () => import('@/views/admin/Contests.vue')
      },
      {
        path: 'contests/new',
        name: 'admin-contest-new',
        meta: { permission: 'contests' },
        component: () => import('@/views/admin/ContestEdit.vue')
      },
      {
        path: 'contests/:id/edit',
        name: 'admin-contest-edit',
        meta: { permission: 'contests' },
        component: () => import('@/views/admin/ContestEdit.vue')
      },
      {
        path: 'submissions',
        name: 'admin-submissions',
        meta: { permission: 'submissions' },
        component: () => import('@/views/admin/Submissions.vue')
      },
      {
        path: 'judges',
        name: 'admin-judges',
        meta: { permission: 'judges' },
        component: () => import('@/views/admin/Judges.vue')
      },
      {
        path: 'announcements',
        name: 'admin-announcements',
        meta: { permission: 'announcements' },
        component: () => import('@/views/admin/Announcements.vue')
      },
      {
        path: 'settings',
        name: 'admin-settings',
        meta: { permission: 'settings' },
        component: () => import('@/views/admin/Settings.vue')
      },
      {
        path: 'backups',
        name: 'admin-backups',
        meta: { permission: 'backups' },
        component: () => import('@/views/admin/Backups.vue')
      },
      {
        path: 'logs',
        name: 'admin-logs',
        meta: { permission: 'logs' },
        component: () => import('@/views/admin/Logs.vue')
      }
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
  const needSuperAdmin = to.matched.some((r) => r.meta.superAdmin)
  const needPermission = to.matched.map((r) => r.meta.permission).find(Boolean)

  if (!store.user && store.token) {
    await store.loadMe()
  }
  if (needAuth && !store.user) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (needAdmin && !store.isAdmin) {
    return { name: 'home' }
  }
  if (needSuperAdmin && !store.isSuperAdmin) {
    return { name: 'admin-dashboard' }
  }
  if (needPermission && !store.hasPermission(needPermission)) {
    return { name: 'admin-dashboard' }
  }
  return true
})

export default router
