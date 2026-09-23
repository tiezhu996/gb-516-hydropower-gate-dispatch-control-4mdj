
import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import './styles.css';
import App from './App.vue';
import { router } from './router';
import { bootstrapAuth } from './hooks/useAuth';

async function mount(): Promise<void> {
  await bootstrapAuth();
  createApp(App).use(createPinia()).use(router).use(ElementPlus).mount('#app');
}

void mount();
