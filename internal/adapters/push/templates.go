package push

const (
	requestTitleES    = "Nueva solicitud de trabajo"
	requestBodyES     = "Recibiste una solicitud de trabajo."
	requestTitleEN    = "New job request"
	requestBodyEN     = "You received a job request."
	messageTitleES    = "Nuevo mensaje"
	messageBodyES     = "Tenés un nuevo mensaje en LoResuelvo."
	messageTitleEN    = "New message"
	messageBodyEN     = "You have a new message in LoResuelvo."
	proposalTitleES   = "Nueva propuesta"
	proposalBodyES    = "Recibiste una propuesta de servicio."
	proposalTitleEN   = "New proposal"
	proposalBodyEN    = "You received a service proposal."
	acceptedTitleES   = "Propuesta aceptada"
	acceptedBodyES    = "Se confirmó una contratación."
	acceptedTitleEN   = "Proposal accepted"
	acceptedBodyEN    = "A service booking was confirmed."
	reminderTitleES   = "Turno próximo"
	reminderBodyES    = "Tenés un servicio programado dentro de las próximas 24 horas."
	reminderTitleEN   = "Upcoming appointment"
	reminderBodyEN    = "You have a service scheduled within the next 24 hours."
	completionTitleES = "Trabajo finalizado"
	completionBodyES  = "El prestador informó la finalización. Revisá el detalle del servicio."
	completionTitleEN = "Work completed"
	completionBodyEN  = "The provider reported completion. Review the service details."
	paymentTitleES    = "Pago final confirmado"
	paymentBodyES     = "Se aprobó el pago del saldo de tu servicio."
	paymentTitleEN    = "Final payment confirmed"
	paymentBodyEN     = "The balance payment for your service was approved."
)

var templates = map[string][2][2]string{
	"job_request_received":               {{requestTitleES, requestBodyES}, {requestTitleEN, requestBodyEN}},
	"conversation.message.created":       {{messageTitleES, messageBodyES}, {messageTitleEN, messageBodyEN}},
	"service_proposal_received":          {{proposalTitleES, proposalBodyES}, {proposalTitleEN, proposalBodyEN}},
	"service_proposal_accepted":          {{acceptedTitleES, acceptedBodyES}, {acceptedTitleEN, acceptedBodyEN}},
	"work_order_close_to_scheduled_time": {{reminderTitleES, reminderBodyES}, {reminderTitleEN, reminderBodyEN}},
	"work_order_completion_reported":     {{completionTitleES, completionBodyES}, {completionTitleEN, completionBodyEN}},
	"work_order_final_payment_approved":  {{paymentTitleES, paymentBodyES}, {paymentTitleEN, paymentBodyEN}},
}
