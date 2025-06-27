resource "aws_db_instance" "transgo_db" {
  identifier             = "transgo-db"
  engine                 = "postgres"
  engine_version         = "15"
  instance_class         = "db.t3.micro"
  allocated_storage      = 20
  storage_type           = "gp2"
  username               = "postgres"
  password               = var.db_password
  parameter_group_name   = "default.postgres15"
  publicly_accessible    = false
  skip_final_snapshot    = true
  vpc_security_group_ids = [var.security_group_id]
}

output "db_endpoint" {
  value = aws_db_instance.transgo_db.endpoint
}

output "db_address" {
  value = aws_db_instance.transgo_db.address
}

output "db_port" {
  value = aws_db_instance.transgo_db.port
}
