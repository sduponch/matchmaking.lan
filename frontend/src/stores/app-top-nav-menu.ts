import { defineStore } from "pinia";

export const useAppTopNavMenuStore = defineStore("appTopNavMenu", () => {
	return [{
		'url': '/',
		'icon': 'bi bi-house-door',
		'text': 'Home'
	}]
});