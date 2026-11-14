// Three route areas: portal (submit), console (handle), admin (manage), plus
// the public login. vue-router was confirmed in issue #5.
import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from './api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('./views/LoginView.vue'),
    },
    {
      path: '/',
      name: 'portal',
      component: () => import('./views/PortalLayout.vue'),
      children: [
        {
          path: '',
          name: 'portal-home',
          component: () => import('./views/portal/Home.vue'),
        },
      ],
    },
    {
      path: '/console',
      name: 'console',
      component: () => import('./views/ConsoleLayout.vue'),
      children: [
        {
          path: '',
          name: 'console-home',
          component: () => import('./views/console/Home.vue'),
        },
      ],
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('./views/AdminLayout.vue'),
      children: [
        {
          path: '',
          name: 'admin-home',
          component: () => import('./views/admin/Home.vue'),
        },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  const publicPages = ['login']
  if (publicPages.includes(to.name as string)) {
    return true
  }
  if (!getToken()) {
    return { name: 'login' }
  }
  return true
})

export default router
