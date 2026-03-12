<template>
	<div>
		<div v-show="show" class="host-panel-right">
			<div class="card d-flex flex-column h-100">
				<div class="card-header d-flex align-items-center justify-content-between">
					<h3 class="card-title">
						<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
							<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
							<path d="M8 9l3 3l-3 3" />
							<path d="M13 15l3 0" />
							<path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
						</svg>
						Console Live
					</h3>
					<div class="d-flex gap-1">
						<button
							@click="copyConsoleOutput"
							class="btn btn-sm btn-ghost-secondary"
							:title="consoleCopied ? 'Copie !' : 'Copier la sortie'"
							:disabled="!currentCommand"
						>
							<svg v-if="!consoleCopied" xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
								<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
								<path d="M8 8m0 2a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2z" />
								<path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2" />
							</svg>
							<svg v-else xmlns="http://www.w3.org/2000/svg" class="icon text-success" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
								<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
								<path d="M5 12l5 5l10 -10" />
							</svg>
						</button>
						<button
							@click="downloadConsoleOutput"
							class="btn btn-sm btn-ghost-secondary"
							title="Telecharger (.txt)"
							:disabled="!currentCommand"
						>
							<svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
								<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
								<path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2" />
								<path d="M7 11l5 5l5 -5" />
								<path d="M12 4l0 12" />
							</svg>
						</button>
						<button
							@click="clearConsoleOutput"
							class="btn btn-sm btn-ghost-secondary"
							title="Vider la console"
							:disabled="!currentCommand"
						>
							<svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
								<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
								<path d="M4 7h16" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" />
								<path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" />
							</svg>
						</button>
						<button @click="hideConsole" class="btn btn-sm btn-ghost-secondary" title="Masquer la console">
							<svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
								<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
								<path d="M17 6l-10 10" />
								<path d="M7 6l10 10" />
							</svg>
						</button>
					</div>
				</div>
				<div class="card-body d-flex flex-column flex-fill p-0" style="min-height: 0;">
					<div v-if="!currentCommand" class="d-flex align-items-center justify-content-center flex-fill text-secondary" style="background: #1e293b; border-radius: 0 0 0.5rem 0.5rem;">
						<div class="text-center p-4">
							<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler mb-2 opacity-50" width="48" height="48" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
								<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
								<path d="M8 9l3 3l-3 3" />
								<path d="M13 15l3 0" />
								<path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
							</svg>
							<div class="opacity-75">Aucune console active</div>
							<div class="small mt-1 opacity-50">Cliquez sur "Voir les logs" pour afficher la sortie d'une commande</div>
						</div>
					</div>

					<div v-else class="d-flex flex-column flex-fill" style="min-width: 0; overflow: hidden;">
						<div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
							<div class="d-flex align-items-start justify-content-between mb-2">
								<div class="flex-fill" style="min-width: 0;">
									<div class="fw-semibold text-light" style="font-size: 0.95rem;">{{ host?.hostname || 'Hote' }}</div>
									<div class="text-secondary small mt-1">
										<code style="background: rgba(0,0,0,0.3); padding: 0.15rem 0.4rem; border-radius: 0.25rem; color: #94a3b8;">{{ currentCommand.prefix }}{{ currentCommand.command }}</code>
									</div>
								</div>
								<span :class="statusClass(currentCommand.status)" class="ms-2">{{ currentCommand.status }}</span>
							</div>
						</div>
						<pre
							ref="consoleOutput"
							class="console-output mb-0 flex-fill"
							v-html="colorizedOutput || '<span style=\'opacity:0.5\'>En attente de sortie...</span>'"
						></pre>
					</div>
				</div>
			</div>
		</div>

		<button
			v-show="!show"
			@click="$emit('show')"
			class="btn btn-primary"
			style="position: fixed; bottom: 1.5rem; right: 1.5rem; z-index: 100;"
		>
			<svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
				<path stroke="none" d="M0 0h24v24H0z" fill="none"/>
				<path d="M8 9l3 3l-3 3" />
				<path d="M13 15l3 0" />
				<path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
			</svg>
			Console
		</button>
	</div>
</template>

<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { useCommandStream } from '../../composables/useCommandStream'
import {
	colorizeConsoleOutput,
	copyConsoleOutput as copyConsoleOutputToClipboard,
	downloadConsoleOutput as downloadConsoleOutputToFile,
} from '../../utils/consoleOutput'

const emit = defineEmits(['hide', 'show', 'update:command', 'history-changed'])

const props = defineProps({
	host: {
		type: Object,
		default: null,
	},
	command: {
		type: Object,
		default: null,
	},
	show: {
		type: Boolean,
		default: false,
	},
})

const auth = useAuthStore()
const consoleCopied = ref(false)
const consoleOutput = ref(null)
const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })

const currentCommand = computed(() => props.command)

const colorizedOutput = computed(() => {
	if (!currentCommand.value) return ''
	return colorizeConsoleOutput(currentCommand.value.output || '')
})

watch(
	() => props.command?.id,
	(commandId) => {
		if (!commandId || !props.show) return
		connectStreamWebSocket(commandId)
	}
)

watch(
	() => props.show,
	(show) => {
		if (!show) {
			closeStream()
			return
		}
		if (props.command?.id) {
			connectStreamWebSocket(props.command.id)
		}
	}
)

function updateCommand(patch) {
	if (!currentCommand.value) return
	emit('update:command', { ...currentCommand.value, ...patch })
}

function copyConsoleOutput() {
	if (!currentCommand.value) return
	copyConsoleOutputToClipboard(currentCommand.value.output || '').then(() => {
		consoleCopied.value = true
		setTimeout(() => {
			consoleCopied.value = false
		}, 2000)
	})
}

function downloadConsoleOutput() {
	if (!currentCommand.value) return
	downloadConsoleOutputToFile(currentCommand.value.output || '', `console-${currentCommand.value.command || 'output'}.txt`)
}

function clearConsoleOutput() {
	updateCommand({ output: '' })
}

function hideConsole() {
	closeStream()
	emit('hide')
}

function connectStreamWebSocket(commandId) {
	openCommandStream(commandId, {
		onInit: (payload) => {
			updateCommand({ status: payload.status, output: payload.output || '' })
			nextTick(() => scrollToBottom())
		},
		onChunk: (payload) => {
			updateCommand({ output: (currentCommand.value?.output || '') + payload.chunk })
			nextTick(() => scrollToBottom())
		},
		onStatus: (payload) => {
			updateCommand({ status: payload.status })
			if (payload.status === 'completed' || payload.status === 'failed') {
				emit('history-changed')
			}
		},
	})
}

function statusClass(status) {
	if (status === 'completed') return 'badge bg-green-lt text-green'
	if (status === 'failed') return 'badge bg-red-lt text-red'
	return 'badge bg-yellow-lt text-yellow'
}

function scrollToBottom() {
	if (consoleOutput.value) {
		consoleOutput.value.scrollTop = consoleOutput.value.scrollHeight
	}
}

onUnmounted(() => {
	closeStream()
})
</script>

<style scoped>
.console-output {
	background: #0f172a;
	color: #f1f5f9;
	padding: 1rem;
	margin: 0;
	overflow-y: auto;
	overflow-x: hidden;
	width: 100%;
	font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
	font-size: 0.813rem;
	line-height: 1.5;
	border-radius: 0 0 0.5rem 0.5rem;
	white-space: pre-wrap;
	word-break: break-all;
}

.host-panel-right {
	width: 38%;
	min-width: 380px;
	display: flex;
	flex-direction: column;
	transition: all 0.3s ease-in-out;
	overflow: hidden;
}

@media (max-width: 991px) {
	.host-panel-right {
		width: 100%;
		min-width: 0;
		max-height: 70vh;
	}
}
</style>
