package nl.theexperts.eyho.domain

import org.jetbrains.exposed.dao.LongEntity
import org.jetbrains.exposed.dao.LongEntityClass
import org.jetbrains.exposed.dao.id.EntityID
import org.jetbrains.exposed.dao.id.LongIdTable
import org.jetbrains.exposed.sql.Column

object Tags:LongIdTable() {
    val name:Column<String> = varchar("name", 20)
    val category = reference("category", Categories)
}

object Categories:LongIdTable() {
    val name:Column<String> = varchar("name",100)
    val description:Column<String> = varchar("description",100)
}

class Category(id: EntityID<Long>): LongEntity(id) {
    companion object: LongEntityClass<Category>(Categories)
    var name by Categories.name
    var description by Categories.description
}

class Tag(id: EntityID<Long>) :LongEntity(id) {
    companion object: LongEntityClass<Tag>(Tags)
    var name by Tags.name
    var category by Category referencedOn Tags.category

}