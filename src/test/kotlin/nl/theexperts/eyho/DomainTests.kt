package nl.theexperts.eyho
import nl.theexperts.eyho.domain.*
import org.jetbrains.exposed.sql.Database
import org.jetbrains.exposed.sql.SchemaUtils
import org.jetbrains.exposed.sql.SizedCollection
import org.jetbrains.exposed.sql.insert
import org.jetbrains.exposed.sql.transactions.transaction
import org.jetbrains.exposed.sql.jodatime.CurrentDateTime
import org.joda.time.DateTime
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.Assertions.assertEquals
import org.springframework.transaction.annotation.Transactional



class DomainTests {

    @Transactional
    @Test
    fun testInsert() {
        Database.connect("jdbc:h2:mem:test;DB_CLOSE_DELAY=-1", driver = "org.h2.Driver", user = "root", password = "")
        transaction {
             SchemaUtils.create(Gatherings, GatheringTags, GatheringParticipants, Users, Tags)

            val tag = Tag.new{
                this.name= "magic"
                this.category = Category.new {
                    this.name = "misc"
                    this.description = "all the is not in the rest"
                }
            }
            val address = Address.new {
                this.city="roosendaal"
                this.nation="nl"
                this.postCode="4701da"
                this.houseNumber=1
                this.streetName="magic"
            }
            val user = User.new {
                this.name = "ruud"
                this.email = "ruud@gmail.com"
                this.address = address
            }
            val gathering = Gathering.new{
                this.name ="test"
                this.organisator = user
                this.tags = SizedCollection(listOf(tag, tag))
                this.date = DateTime.now()
            }
            assertEquals(2, gathering.tags.count())
        }

        transaction {
            assertEquals(Gathering.all().count(), 1)
            val gathering = Gathering.find{Gatherings.id eq Integer.toUnsignedLong(1)}.first()
            assertEquals(gathering.name, "test")
        }
    }

}

