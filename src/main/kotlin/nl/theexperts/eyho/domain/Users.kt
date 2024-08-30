package nl.theexperts.eyho.domain

import org.jetbrains.exposed.dao.LongEntity
import org.jetbrains.exposed.dao.LongEntityClass
import org.jetbrains.exposed.dao.id.EntityID
import org.jetbrains.exposed.dao.id.LongIdTable
import org.jetbrains.exposed.sql.Column

object Users : LongIdTable() {
    val name: Column<String> = varchar(name = "name", length = 30)
    val email: Column<String?> = varchar(name = "email", length = 30).nullable()
    val address = reference("address", Addresses).nullable()

    init {
        index(true, name, email)
    }
}

class User(id: EntityID<Long>) : LongEntity(id) {
    companion object : LongEntityClass<User>(Users)
    var name by Users.name
    var email by Users.email
    var address by Address optionalReferencedOn Users.address
}

object Addresses:LongIdTable() {
    val nation:Column<String> = varchar("nation",100)
    val postCode:Column<String?> = varchar("postcode",100).nullable()
    val city:Column<String?> = varchar("city",100).nullable()
    val streetName:Column<String?> = varchar("streetName", length = 100).nullable()
    val houseNumber:Column<Int?> = integer("houseNumber").nullable()
}

class Address(id:EntityID<Long>) : LongEntity(id) {
    companion object: LongEntityClass<Address>(Addresses)
    var nation by Addresses.nation
    var postCode by Addresses.postCode
    var city by Addresses.city
    var streetName by Addresses.streetName
    var houseNumber by Addresses.houseNumber
}

