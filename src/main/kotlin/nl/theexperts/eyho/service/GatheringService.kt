package nl.theexperts.eyho.service

import nl.theexperts.eyho.domain.Gathering
import nl.theexperts.eyho.domain.GatheringParticipants
import nl.theexperts.eyho.domain.User
import org.jetbrains.exposed.sql.*
import org.jetbrains.exposed.sql.SqlExpressionBuilder.eq
import org.jetbrains.exposed.sql.transactions.transaction

import org.springframework.stereotype.Component
import org.springframework.transaction.annotation.Transactional

@Component
@Transactional
class GatheringService {

    @Transactional
    fun findGatheringById(id: Long): Gathering? {
        var gathering: Gathering? = null
        transaction {
            gathering = Gathering.findById(id)
        }
        return gathering
    }

    @Transactional
    fun createGathering(gathering: Gathering) {
        transaction {

        }
    }

    @Transactional
    fun addParticipant(aGathering: Gathering, newParticipant: User) {
        transaction {
            val isAlreadyIncluded = aGathering.participants.any {
                it.id == newParticipant.id
            }
            if (isAlreadyIncluded) {
                throw Exception("Participant is already included")
            }

            GatheringParticipants.insert {
                it[participant] = newParticipant.id
                it[gathering] = aGathering.id
            }
        }
    }

    @Transactional
    fun removeParticipant(gathering: Gathering, participant: User) {
        transaction {
            val isAlreadyIncluded = gathering.participants.any {
                it.id == participant.id
            }
            if (!isAlreadyIncluded) {
                throw Exception("Participant is not yet included in Gathering!")
            }

            GatheringParticipants.deleteWhere {
                GatheringParticipants.participant eq participant.id
                GatheringParticipants.gathering eq gathering.id
            }
        }

    }

    data class GatheringRequest (
        val name:String,
        val date:String,
        val organisationId:Long,
    )

}