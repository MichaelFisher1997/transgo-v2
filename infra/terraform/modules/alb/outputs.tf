output "target_group_arn" {
  description = "The ARN of the target group"
  value       = aws_lb_target_group.transgo.arn
}

output "alb_dns_name" {
  description = "The DNS name of the ALB"
  value       = aws_lb.transgo.dns_name
}

output "security_group_id" {
  description = "Security group ID for ALB"
  value       = aws_security_group.alb.id
}

output "alb_dns_name" {
  description = "DNS name of the ALB"
  value       = aws_lb.transgo.dns_name
}
