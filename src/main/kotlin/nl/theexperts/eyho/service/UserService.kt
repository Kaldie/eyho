package nl.theexperts.eyho.service

import nl.theexperts.eyho.domain.Address
import nl.theexperts.eyho.domain.Addresses
import nl.theexperts.eyho.domain.User
import nl.theexperts.eyho.domain.Users
import org.jetbrains.exposed.sql.SqlExpressionBuilder.eq
import org.jetbrains.exposed.sql.transactions.transaction
import org.springframework.stereotype.Component
import org.springframework.transaction.annotation.Transactional

@Component
@Transactional
class UserService {

    @Transactional
    fun createUser(request: UserRequest) {
        transaction {
            User.new {
                this.name = request.name
                this.email = request.email
                if (request.address != null)
                    address = Address.new {
                        nation = request.address.nation
                        postCode = request.address.postCode
                        city = request.address.city
                        streetName = request.address.streetName
                        houseNumber = request.address.houseNumber
                    }
            }
        }
    }

    @Transactional
    fun findUserByID(id: Long): User? {
        var user: User? = null
        transaction {
            user = User.findById(id)
        }
        return user
    }

    @Transactional
    fun findUserByName(name: String): User? {
        var user: User? = null
        transaction {
            val users = User.find(
                Users.name eq name
            )
            if (users.count() != 1L) {
                throw Exception("Found 0 or more then 1 user with the same name!")
            }

            user = users.first()
        }
        return user
    }

    @Transactional
    fun findUserByNameAndEmail(name: String, email: String): User? {
        var user: User? = null
        transaction {
            user = User.find {
                Users.name eq name
                Users.email eq email
            }.first()
        }
        return user
    }


    data class UserRequest(
        val name: String,
        val email: String?,
        val address: AddressRequest?

    )

    data class AddressRequest(
        val nation: String,
        val streetName: String?,
        val city: String?,
        val postCode: String?,
        val houseNumber: Int?,
    )


}