import { createRouter, createWebHistory } from 'vue-router'
import AccountView from './views/AccountView.vue'
import SignInView from './views/SignInView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/account' },
    { path: '/sign-in', name: 'sign-in', component: SignInView },
    { path: '/account', name: 'account', component: AccountView },
  ],
})
