package cli

import "strings"

type messages map[string]string

const legalNoticeText = "Use kali-tools only on systems you own or are explicitly authorized in writing to test. Active scans send traffic to up to 256 hosts x 32 ports. The tool keeps no cloud telemetry. Run `kali-tools legal --accept` to record that you have read this."

var portuguese = messages{
	legalNoticeText: "Use o kali-tools apenas em sistemas que você possui ou que tenha autorização expressa, por escrito, para testar. Varreduras ativas enviam tráfego para até 256 hosts x 32 portas. A ferramenta não envia telemetria para a nuvem. Rode `kali-tools legal --accept` para registrar que você leu isto.",
	"version":       "versão",
	"TLS audit":     "Auditoria TLS", "verified": "verificado", "accepted": "aceito", "rejected": "rejeitado", "client_unsupported": "não suportado pelo cliente", "fatal": "fatal", "warn": "aviso", "info": "informação",
	"neighbors": "vizinhos", "INTERFACE": "INTERFACE", "FAMILY": "FAMÍLIA", "ADDRESS": "ENDEREÇO", "ROUTER": "ROTEADOR",
	"LOCAL": "LOCAL", "REMOTE": "REMOTO", "PROCESS": "PROCESSO", "endpoint": "endpoint", "duration_ms": "duração_ms", "byte_count": "octetos", "source": "origem",
	"CAPABILITY": "CAPACIDADE", "STATE": "ESTADO", "PROVIDER": "PROVEDOR", "DETAIL": "DETALHE",
	"NAME": "NOME", "INDEX": "ÍNDICE", "UP": "ATIVA", "MTU": "MTU", "MAC": "MAC", "ADDRESSES": "ENDEREÇOS",
	"ADAPTER": "ADAPTADOR", "GATEWAYS": "GATEWAYS", "DNS": "DNS", "COUNTER": "CONTADOR", "RECEIVED": "RECEBIDO", "SENT": "ENVIADO",
	"HOST": "HOST", "PORT": "PORTA", "SERVICE": "SERVIÇO", "METHOD": "MÉTODO", "VERSION": "VERSÃO", "PATH": "CAMINHO",
	"ID": "ID", "CREATED": "CRIADO", "KIND": "TIPO", "SUMMARY": "RESUMO",
	"NO": "Nº", "TIME": "HORA", "LENGTH": "TAMANHO", "SOURCE": "ORIGEM", "DESTINATION": "DESTINO", "PROTOCOL": "PROTOCOLO", "INFO": "INFO",
	"System": "Sistema", "language": "idioma", "retention_days": "dias_de_retenção", "state": "estado", "key": "chave",
	"Ubuntu 24.04 LTS is the documented Linux baseline. Debian 13 and Kali are best-effort and come with no guarantee.": "Ubuntu 24.04 LTS é a base Linux documentada. Debian 13 e Kali são melhor esforço e não têm garantia.",
	"SSID": "SSID", "BSSID": "BSSID", "profile": "perfil", "signal": "sinal", "authentication": "autenticação", "channel": "canal",
	"id": "id", "kind": "tipo", "created": "criado", "finished": "finalizado", "summary": "resumo",
	"available": "disponível", "completed": "concluído", "open": "aberta", "connected": "conectado", "failed": "falhou", "closed": "fechada",
	"queued": "na fila", "running": "executando", "cancelled": "cancelado", "unsupported": "não suportado", "dependency_missing": "dependência ausente",
	"permission_required": "permissão necessária", "location_consent_required": "consentimento de localização necessário", "disconnected": "desconectado",
	"invalid_input": "entrada inválida", "not_found": "não encontrado", "timeout": "tempo esgotado", "conflict": "conflito", "unauthorized": "não autorizado", "forbidden": "proibido", "unavailable": "indisponível", "certificate_invalid": "certificado inválido", "no_connection": "sem conexão",
	"filtered": "filtrada", "unreachable": "inalcançável", "up": "ativo", "down": "inativo", "unknown": "desconhecido", "RTT": "RTT", "HOST_STATE": "ESTADO_HOST",
	"partial results: the scan deadline was reached": "parcial: o prazo da varredura foi atingido e a lista está incompleta",
	"error": "erro", "saved": "salvo", "packets": "pacotes", "format": "formato", "limited": "limitado",
	"DNS inspection": "Inspeção DNS", "queries": "consultas", "agree": "concorda", "verdict": "veredito", "dnssec": "dnssec",
	"responders": "respondedores", "out of scope": "fora do escopo", "malformed": "malformado", "SNMP": "SNMP",
	"TYPE": "TIPO", "DESCR": "DESCRIÇÃO", "STATUS": "ESTADO",
	"Open":                   "Abrir",
	"cancellation requested": "cancelamento solicitado", "history cleared (queued and running jobs are kept)": "histórico limpo (jobs na fila e em execução são mantidos)",
	"warning: the saved Wi-Fi key will be printed to standard output. Terminal scrollback, shell logs and SSH session recordings may retain it.": "aviso: a chave Wi-Fi salva será impressa na saída padrão. O histórico do terminal, logs do shell e gravações de sessões SSH podem retê-la.",
}

func label(lang, key string) string {
	if lang == "pt-BR" {
		if value, ok := portuguese[key]; ok {
			return value
		}
	}
	return key
}

func headers(lang, value string) string {
	parts := strings.Fields(value)
	for i := range parts {
		parts[i] = label(lang, parts[i])
	}
	return strings.Join(parts, "\t")
}
