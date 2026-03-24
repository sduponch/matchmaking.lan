<script setup lang="ts">
import SidebarNav from '@/components/app/SidebarNav.vue';
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth';

interface MenuItem {
	icon?: string
	text?: string
	url?: string
	label?: string
	highlight?: boolean
	accessGroup?: string[]
	children?: MenuItem[]
}

const props = defineProps<{ menu: MenuItem }>();

const auth = useAuthStore();

function checkAccess(menu: MenuItem): boolean {
	if (!menu.accessGroup || menu.accessGroup.length === 0) return true;
	if (!auth.user) return false;
	return menu.accessGroup.includes(auth.user.role);
}

function subIsActive(items: MenuItem[]): boolean {
	const currentRoute = useRouter().currentRoute.value.path;
	return items.some(item => item.url === currentRoute);
}
</script>
<template>
	<!-- menu with submenu -->
	<div v-if="menu.children && checkAccess(menu)" class="menu-item has-sub" :class="{ 'active': subIsActive(menu.children) }">
		<a class="menu-link">
			<span class="menu-icon" v-if="menu.icon">
				<i :class="menu.icon"></i>
				<span class="w-5px h-5px rounded-3 bg-theme position-absolute top-0 end-0 mt-3px me-3px" v-if="menu.highlight"></span>
			</span>
			<span class="menu-text">{{ menu.text }}</span>
			<span class="menu-caret"><b class="caret"></b></span>
		</a>
		<div class="menu-submenu">
			<template v-for="submenu in menu.children">
				<sidebar-nav v-if="checkAccess(submenu)" :menu="submenu"></sidebar-nav>
			</template>
		</div>
	</div>

	<!-- menu without submenu -->
	<router-link v-else-if="!menu.children && checkAccess(menu)" :to="menu.url as string" custom v-slot="{ navigate, href, isActive }">
		<div class="menu-item" :class="{ 'active': isActive }">
			<a :href="href" @click="navigate" class="menu-link">
				<span class="menu-icon" v-if="menu.icon">
					<i :class="menu.icon"></i>
					<span class="menu-icon-label" v-if="menu.label">{{ menu.label }}</span>
				</span>
				<span class="menu-text">{{ menu.text }}</span>
			</a>
		</div>
	</router-link>
</template>
