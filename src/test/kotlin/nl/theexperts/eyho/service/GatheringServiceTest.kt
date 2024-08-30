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


class GatheringServiceTest {

    @Test
    fun testFindById() {
        transaction {
            val gathering = GatheringService().findGatheringById(1)
            assertNotNull(gathering)

            val id = gathering?.id?.value ?: 0L
            val name = gathering?.name ?: "not set"
            val participant = gathering?.participants?.first()
            assertEquals(1L, id)
            assertEquals(gatheringName, name)
            assertEquals(participantName, participant?.name)
        }
    }

    @Test
    fun testAddParticipant() {
        val service = GatheringService()
        val gathering = service.findGatheringById(1)
        var newParticipant: User? = null

        transaction {
            newParticipant = User.new {
                name = "Freek"
            }
        }

        gathering ?: throw IllegalArgumentException("name expected")
        newParticipant.let {
            it ?: throw IllegalArgumentException("participant expected")
            service.addParticipant(gathering, it)
        }

        val newGathering = service.findGatheringById(1L)

        transaction {
            newGathering?.load(Gathering::participants)
            assertEquals(2L, newGathering?.participants?.count())
            assertTrue(newGathering?.participants?.any {
                it.id == newParticipant?.id
            } ?: false)
        }
    }

    @Test
    fun testIfThrowsIfParticipantAlreadyExists() {
        val service = GatheringService()
        val gathering = service.findGatheringById(1)
        gathering ?: throw Exception("Gathering should exists")

        transaction {
            gathering.load(Gathering::participants)
//          Test if there is at least one participant
            assertEquals(1L, gathering.participants.count())
            org.junit.jupiter.api.assertThrows<Exception> {
                service.addParticipant(gathering, gathering.participants.first())
            }
        }
    }

    @Test
    fun testRemoveParticipant() {
        val service = GatheringService()
        var gathering = service.findGatheringById(1)

        transaction {
            gathering ?: throw Exception("Gathering should exists")
            gathering?.load(Gathering::participants)
            val currentCount = gathering?.participants?.count() ?: 0
            assertEquals(1, currentCount)
            val participant = gathering?.participants?.first()
            participant ?: throw Exception("Gathering should exists")

            service.removeParticipant(gathering!!, participant)
            gathering = gathering?.load(Gathering::participants)
            assertEquals(currentCount - 1, gathering?.participants?.count())
        }
    }

    @Test
    fun testRemoveParticipantThrowsWhenUnknown() {
        val service = GatheringService()
        val gathering = service.findGatheringById(1)
        gathering ?: throw Exception("lalal")
        org.junit.jupiter.api.assertThrows<Exception> {
            service.removeParticipant(gathering, User.new {
                name = "lala"
            })
        }
    }

    //    companion object {
    private val gatheringName = "gathering"
    private val organisatorName = "piet"
    private val participantName = "jan"

    @BeforeEach
    fun beforeEach() {
        Database.connect(
            "jdbc:h2:mem:test;DB_CLOSE_DELAY=-1", driver = "org.h2.Driver", user = "root", password = ""
        )
        transaction {
            SchemaUtils.create(Gatherings, GatheringTags, GatheringParticipants, Users, Tags)

            Gathering.new {
                name = gatheringName
                date = DateTime.now()
                organisator = User.new {
                    name = organisatorName
                }
                participants = SizedCollection(listOf(User.new {
                    name = participantName
                }))
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





