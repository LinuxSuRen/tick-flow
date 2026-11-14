// Three route areas: portal (submit), console (handle), admin (manage),
// plus the public pages. vue-router was confirmed in issue #5.
import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from './api'
import { meStore } from './stores/me'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('./views/LoginView.vue') },
    { path: '/setup', name: 'setup', component: () => import('./views/SetupView.vue') },
    {
      path: '/accept-invitation',
      name: 'accept-invitation',
      component: () => import('./views/AcceptInvitationView.vue'),
    },
    {
      path: '/',
      name: 'portal',
      component: () => import('./views/PortalLayout.vue'),
      children: [
        { path: '', name: 'portal-home', component: () => import('./views/portal/Home.vue') },
        { path: 'new/:typeId', name: 'portal-new', component: () => import('./views/portal/NewTicket.vue') },
        { path: 'success/:ticketId', name: 'portal-success', component: () => import('./views/portal/Success.vue') },
        { path: 'mine', name: 'portal-mine', component: () => import('./views/portal/Mine.vue') },
        {
          path: 'mine/:id',
          name: 'portal-ticket',
          component: () => import('./views/portal/TicketPage.vue'),
        },
      ],
    },
    {
      path: '/console',
      name: 'console',
      component: () => import('./views/ConsoleLayout.vue'),
      children: [
        { path: '', name: 'console-list', component: () => import('./views/console/List.vue') },
        {
          path: 'ticket/:id',
          name: 'console-ticket',
          component: () => import('./views/console/TicketPage.vue'),
        },
      ],
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('./views/AdminLayout.vue'),
      children: [
        { path: '', redirect: '/admin/members' },
        { path: 'members', name: 'admin-members', component: () => import('./views/admin/Members.vue') },
        { path: 'types', name: 'admin-types', component: () => import('./views/admin/Types.vue') },
        { path: 'branding', name: 'admin-branding', component: () => import('./views/admin/Branding.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

const publicPages = new Set(['login', 'setup', 'accept-invitation'])

router.beforeEach((to) => {
  if (publicPages.has(to.name as string)) {
    return true
  }
  if (!getToken()) {
    return { name: 'login' }
  }
  // Console and admin need a staff role; the backend enforces it too.
  if (String(to.path).startsWith('/console') && !['agent', 'tenant_admin'].includes(meStore.role)) {
    return { name: 'portal-home' }
  }
  if (String(to.path).startsWith('/admin') && meStore.role !== 'tenant_admin') {
    return { name: 'portal-home' }
  }
  return true
})

export default router
