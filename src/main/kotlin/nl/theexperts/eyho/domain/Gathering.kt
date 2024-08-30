package nl.theexperts.eyho.domain

import org.jetbrains.exposed.dao.LongEntity
import org.jetbrains.exposed.dao.LongEntityClass
import org.jetbrains.exposed.dao.id.EntityID
import org.jetbrains.exposed.dao.id.IntIdTable
import org.jetbrains.exposed.dao.id.LongIdTable
import org.jetbrains.exposed.sql.Column
import org.jetbrains.exposed.sql.ReferenceOption
import org.jetbrains.exposed.sql.jodatime.*

object Gatherings : LongIdTable() {
    val name: Column<String> = varchar("name",20)
    val date = date("date").nullable()
    val organisator = reference("organisator",Users)
}

class Gathering(id: EntityID<Long>) : LongEntity(id) {
    companion object : LongEntityClass<Gathering>(Gatherings)
    var name by Gatherings.name
    var date by Gatherings.date
    var organisator by User referencedOn Gatherings.organisator
    var participants by User via GatheringParticipants
    var tags by Tag via GatheringTags
}

enum class ParticipationStatus(val status: String) {
    Open("open"),
    Close("close"),
    InProgress("in_progress")
}

object ParticipationStatusTable:IntIdTable("ParticipationStatus") {
    val status = enumeration<ParticipationStatus>("status", ParticipationStatus::class) // will create integer column
}

object GatheringTags : LongIdTable() {
    val gathering = reference("gathering", Gatherings)
    val tag = reference("tag", Tags)
}

object GatheringParticipants : LongIdTable() {
    val gathering = reference("gathering", Gatherings)
    val participant = reference("participant", Users)
    val state = reference("state", ParticipationStatusTable).nullable()
}

class GatheringParticipant(id:EntityID<Long>):LongEntity(id) {
    companion object : LongEntityClass<GatheringParticipant>(GatheringParticipants)
    var gathering by Gathering referencedOn GatheringParticipants.gathering
    var participant by User referencedOn GatheringParticipants.participant

}