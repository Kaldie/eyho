package nl.theexperts.eyho

import org.jetbrains.exposed.spring.autoconfigure.ExposedAutoConfiguration
import org.springframework.boot.autoconfigure.ImportAutoConfiguration
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication

@SpringBootApplication
// ExposedAutoConfiguration is a Spring Boot auto-configuration class that configures Exposed.
@ImportAutoConfiguration(ExposedAutoConfiguration::class)
class SpringApplication

fun main(args: Array<String>) {
	@Suppress("SpreadOperator")
	runApplication<SpringApplication>(*args)
}





