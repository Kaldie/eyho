package nl.theexperts.eyho.service

import nl.theexperts.eyho.domain.*
import org.jetbrains.exposed.dao.load
import org.jetbrains.exposed.sql.*
import org.jetbrains.exposed.sql.transactions.transaction
import org.joda.time.DateTime
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions.*
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test


class UserServiceTest {

    @Test
    fun testFindById() {
        transaction {
            val service = UserService()
            val user = service.findUserByID(1)
            assertNotNull(user)
            assertEquals(user?.name, aname)
            assertEquals(user?.email, aemail)

            user?.load(User::address)
            assertNotNull(user?.address)
        }
    }

    @Test
    fun testFindUserByName() {
        val service = UserService()
        val user = service.findUserByName(aname)
        assertEquals(aname, user?.name)
        assertEquals(aemail, user?.email)
    }

    @Test
    fun testFindByNameFailsWithMultipleUsers() {
        transaction {
            User.new {
                this.name = aname
                this.email = aemail + "something"
            }
        }

        val service = UserService()

        org.junit.jupiter.api.assertThrows<Exception> {
            service.findUserByName(aname)
        }
    }

    @Test
    fun testFindByNameFailsWithNoUsers() {
        val service = UserService()

        org.junit.jupiter.api.assertThrows<Exception> {
            service.findUserByName(aname + "lala")
        }
    }

    @Test
    fun testCreateUserWithRequest() {
        var numberOfUsers: Long? = null
        transaction {
            numberOfUsers = User.count()
        }
        val service = UserService()
        service.createUser(
            UserService.UserRequest(
                aname + "something",
                aemail,
                UserService.AddressRequest(
                    anation,
                    acity,
                    astreetName,
                    apostCode,
                    ahouseNumber,
                )
            )
        )
        val expectedUsers = (numberOfUsers?.plus(1)) ?: 0
        transaction {
            assertEquals(expectedUsers, User.count())
        }
    }

    private val aname = "name"
    private val aemail = "email"
    private val anation = "nation"
    private val acity = "city"
    private val astreetName = "streetName"
    private val apostCode = "1231AB"
    private val ahouseNumber = 1


    @BeforeEach
    fun beforeEach() {
        Database.connect(
            "jdbc:h2:mem:test;DB_CLOSE_DELAY=-1", driver = "org.h2.Driver", user = "root", password = ""
        )
        transaction {
            SchemaUtils.create(Gatherings, GatheringTags, GatheringParticipants, Users, Tags)

            User.new {
                this.name = aname
                this.email = aemail
                address = Address.new {
                    this.nation = anation
                    this.city = acity
                    this.streetName = astreetName
                    this.houseNumber = ahouseNumber
                }
            }
        }
    }

    @AfterEach
    fun afterEach() {
        val db = Database.connect(
            "jdbc:h2:mem:test;DB_CLOSE_DELAY=-1", driver = "org.h2.Driver", user = "root", password = ""
        )
        transaction {
            SchemaUtils.drop(Gatherings, GatheringTags, GatheringParticipants, Users, Tags)
        }
    }

}





